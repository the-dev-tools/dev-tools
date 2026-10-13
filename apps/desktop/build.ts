import { Command } from '@effect/platform';
import { NodeContext } from '@effect/platform-node';
import { Config, Effect, pipe } from 'effect';
import { build, type Configuration } from 'electron-builder';
import { chmodSync, copyFileSync, existsSync, mkdirSync, statSync } from 'node:fs';
import { createRequire } from 'node:module';
import nodePath from 'node:path';

const libFiles = (lib: string) => [`node_modules/${lib}/package.json`, `node_modules/${lib}/dist`];

/*
 * The stresseur binary (record / generate), shipped in Resources as
 * `stresseur` (`stresseur.exe` on Windows); src/main/record.ts runs it from
 * process.resourcesPath. Source: $STRESSEUR_BIN (a built binary), else the npm
 * platform package the `stresseur` launcher uses: `stresseur-<os>-<arch>` or
 * `@stresseur/win32-<arch>`, file `bin/stress(.exe)`. Its version is pinned by
 * the `stresseur` devDependency (pnpm-workspace.yaml catalog). `dev` does not run this
 * file; it finds the binary through STRESSEUR_BIN or PATH.
 */
const STRESSEUR_EXE = process.platform === 'win32' ? 'stresseur.exe' : 'stresseur';

const findStresseurBinary = () => {
  const fromEnv = process.env['STRESSEUR_BIN'];
  if (fromEnv) {
    if (!existsSync(fromEnv) || !statSync(fromEnv).isFile()) throw new Error(`STRESSEUR_BIN is not a file: ${fromEnv}`);
    return fromEnv;
  }

  const pkg =
    process.platform === 'win32' ? `@stresseur/win32-${process.arch}` : `stresseur-${process.platform}-${process.arch}`;
  const file = `${pkg}/bin/${process.platform === 'win32' ? 'stress.exe' : 'stress'}`;
  const require = createRequire(import.meta.url);
  const candidates = [
    () => require.resolve(file),
    () => createRequire(require.resolve('stresseur/package.json')).resolve(file),
  ];
  for (const candidate of candidates) {
    try {
      return candidate();
    } catch {
      // Try the next place.
    }
  }

  throw new Error(
    `No stresseur binary for ${process.platform}-${process.arch}. Set STRESSEUR_BIN to a built binary, ` +
      `or install the npm package ${pkg}.`,
  );
};

/** Copies the binary to dist/resources with its final name and the execute bit. */
const stageStresseurBinary = () => {
  const source = findStresseurBinary();
  const target = nodePath.join(import.meta.dirname, 'dist', 'resources', STRESSEUR_EXE);
  mkdirSync(nodePath.dirname(target), { recursive: true });
  copyFileSync(source, target);
  chmodSync(target, 0o755);
  console.log(`Bundling stresseur from ${source}`);
  return target;
};

const stresseurBinary = stageStresseurBinary();

const config: Configuration = {
  // electron-builder keeps file modes, but make sure the packed binary is executable (Linux AppImage).
  afterPack: (context) => {
    const resources =
      context.electronPlatformName === 'darwin'
        ? nodePath.join(context.appOutDir, `${context.packager.appInfo.productFilename}.app`, 'Contents', 'Resources')
        : nodePath.join(context.appOutDir, 'resources');
    const packed = nodePath.join(resources, STRESSEUR_EXE);
    if (!existsSync(packed)) throw new Error(`The stresseur binary is missing from the package: ${packed}`);
    if (context.electronPlatformName !== 'win32') chmodSync(packed, 0o755);
    return Promise.resolve();
  },
  artifactName: '${productName}-${version}-${platform}-${arch}.${ext}',
  extraMetadata: {
    name: 'DevTools-Studio',
  },
  // Signed on Windows by signtoolOptions.sign (electron-builder signs .exe extra resources).
  extraResources: [{ from: stresseurBinary, to: STRESSEUR_EXE }],
  files: ['!**/*', 'out', ...libFiles('@the-dev-tools/server'), ...libFiles('@the-dev-tools/worker-js')],
  linux: {
    category: 'Development',
    target: ['AppImage'],
  },
  mac: {
    // Sign the bundled stresseur binary with the app (hardened runtime, notarization).
    binaries: [`Contents/Resources/${STRESSEUR_EXE}`],
    category: 'public.app-category.developer-tools',
    entitlements: 'build/entitlements.mac.plist',
    entitlementsInherit: 'build/entitlements.mac.plist',
    gatekeeperAssess: false,
    hardenedRuntime: true,
    type: 'distribution',
  },
  npmRebuild: false,
  nsis: {
    allowToChangeInstallationDirectory: true,
    oneClick: false,
  },
  publish: { provider: 'custom' },
  win: {
    signtoolOptions: {
      sign: (configuration) =>
        pipe(
          Effect.gen(function* () {
            const exitCode = yield* pipe(
              Command.make(
                'azuresigntool',
                'sign',
                '--timestamp-rfc3161',
                'http://timestamp.globalsign.com/tsa/advanced',
                '--azure-key-vault-tenant-id',
                yield* Config.string('AZURE_KEY_VAULT_TENANT_ID'),
                '--azure-key-vault-url',
                yield* Config.string('AZURE_KEY_VAULT_URL'),
                '--azure-key-vault-client-id',
                yield* Config.string('AZURE_KEY_VAULT_CLIENT_ID'),
                '--azure-key-vault-client-secret',
                yield* Config.string('AZURE_KEY_VAULT_CLIENT_SECRET'),
                '--azure-key-vault-certificate',
                yield* Config.string('AZURE_KEY_VAULT_CERTIFICATE'),
                configuration.path,
              ),
              Command.stdout('inherit'),
              Command.stderr('inherit'),
              Command.exitCode,
            );
            // azuresigntool reports failures (e.g. an expired Key Vault client
            // secret) through its exit code only; without this check the build
            // shipped unsigned Windows installers from 1.0.0 to 1.1.3.
            if (exitCode !== 0) {
              return yield* Effect.fail(
                new Error(`azuresigntool exited with code ${exitCode} while signing ${configuration.path}`),
              );
            }
          }),
          Effect.provide(NodeContext.layer),
          Effect.runPromise,
        ),
    },
  },
};

await build({ config, publish: 'never' });
