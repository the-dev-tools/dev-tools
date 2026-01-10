import { Command } from '@effect/platform';
import { NodeContext } from '@effect/platform-node';
import { Config, Effect, pipe } from 'effect';
import { build, type Configuration } from 'electron-builder';
import { fileURLToPath } from 'node:url';

const config: Configuration = {
  artifactName: '${productName}-${version}-${platform}-${arch}.${ext}',
  asarUnpack: ['resources/**', '**/node_modules/@the-dev-tools/server/dist/server', '**/node_modules/@the-dev-tools/cli/dist/cli'],
  extraMetadata: {
    name: 'DevTools',
  },
  files: ['!src/*', '!*.{js,ts}', '!{tsconfig.json,tsconfig.*.json}'],
  icon: pipe(import.meta.resolve('@the-dev-tools/client/assets/favicon/favicon.png'), fileURLToPath),
  linux: {
    category: 'Development',
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
