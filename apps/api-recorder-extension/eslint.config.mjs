import { eslintReactConfig } from '@the-dev-tools/config-eslint';
import * as TS from 'typescript-eslint';

export default TS.config(
  ...eslintReactConfig,
  {
    rules: {
      // https://github.com/typescript-eslint/typescript-eslint/issues/9902
      // https://github.com/typescript-eslint/typescript-eslint/issues/9899
      // https://github.com/microsoft/TypeScript/issues/59792
      '@typescript-eslint/no-deprecated': 'off',
    },
  },
);
