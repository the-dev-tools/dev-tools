import { pipe } from 'effect';
import { build, type Configuration } from 'electron-builder';
import { fileURLToPath } from 'node:url';

const config: Configuration = {
  artifactName: '${productName}-${version}-${platform}-${arch}.${ext}',
  asarUnpack: ['resources/**'],
  extraMetadata: {
    name: 'dev-tools',
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
    signAndEditExecutable: false,
  },
};

await build({ config, publish: 'never' });
