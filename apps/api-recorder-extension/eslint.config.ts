import { Config } from 'typescript-eslint';

import { eslintReactConfig } from '@the-dev-tools/config-eslint';

export default [
  ...eslintReactConfig,
  {
    rules: {
      // https://github.com/typescript-eslint/typescript-eslint/issues/9902
      // https://github.com/typescript-eslint/typescript-eslint/issues/9899
      // https://github.com/microsoft/TypeScript/issues/59792
      '@typescript-eslint/no-deprecated': 'off',
      'import-x/no-unresolved': 'off',
    },
  },
] satisfies Config;
