import { Command } from '@effect/platform';
import { NodeContext } from '@effect/platform-node';
import { Config, Effect, pipe } from 'effect';
import { build, type Configuration } from 'electron-builder';

const libFiles = (lib: string) => [`node_modules/${lib}/package.json`, `node_modules/${lib}/dist`];

/*
 * Naming: the app is displayed as "Stresseur Studio", but everything that
 * identifies an existing install keeps its DevTools-Studio value, so upgrades
 * happen in place. Do not change the pinned values below.
 *
 * - appId: was implicit (`com.electron.` + lowercased `name`). It is the macOS
 *   bundle ID, and on Windows it seeds the NSIS GUID (uninstall key, upgrade
 *   detection) and the AppUserModelID (taskbar pins).
 * - executableName: `DevTools-Studio.exe` / `DevTools-Studio.app` / `devtools-studio`.
 *   It also names the Windows install folder, so existing shortcuts, pinned
 *   taskbar items and the in-place upgrade keep pointing at the same path.
 * - artifactName: release asset names are read by the auto-updater metadata,
 *   README, install docs and the dev.tools download page. Hardcoded prefix,
 *   because `${productName}` would now expand to "Stresseur Studio".
 * - extraMetadata.name: keep; it feeds the updater cache folder
 *   (`devtools-studio-updater`) and the default appId.
 * - extraMetadata.productName: the display name. Electron derives the
 *   user-data folder from it, which is why `migrateDataDir` (src/main)
 *   moves "DevTools-Studio" into "Stresseur Studio" at startup.
 */
const config: Configuration = {
  appId: 'com.electron.devtools-studio',
  artifactName: 'DevTools-Studio-${version}-${platform}-${arch}.${ext}',
  executableName: 'DevTools-Studio',
  extraMetadata: {
    name: 'DevTools-Studio',
    productName: 'Stresseur Studio',
  },
  files: ['!**/*', 'out', ...libFiles('@the-dev-tools/server'), ...libFiles('@the-dev-tools/worker-js')],
  linux: {
    category: 'Development',
    // Linux lowercases the executable by default; pin today's name explicitly.
    executableName: 'devtools-studio',
    target: ['AppImage'],
  },
  mac: {
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
            yield* pipe(
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
          }),
          Effect.provide(NodeContext.layer),
          Effect.runPromise,
        ),
    },
  },
};

await build({ config, publish: 'never' });
