import { Config } from 'typescript-eslint';

import base from '@the-dev-tools/config-eslint';

export default [
  ...base,
  {
    ignores: ['apps', 'libraries'],
  },
] satisfies Config;
