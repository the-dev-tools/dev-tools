import { ConfigArray } from 'typescript-eslint';

import base from '@the-dev-tools/eslint-config';

const config: ConfigArray = [
  ...base,
  {
    rules: {
      // https://github.com/typescript-eslint/typescript-eslint/issues/9902
      // https://github.com/typescript-eslint/typescript-eslint/issues/9899
      // https://github.com/microsoft/TypeScript/issues/59792
      '@typescript-eslint/no-deprecated': 'off',
      'import-x/no-unresolved': 'off',
    },
  },
];

export default config;
