import { ConfigArray } from 'typescript-eslint';

import base from '@the-dev-tools/eslint-config';

const config: ConfigArray = [
  ...base,
  {
    ignores: ['apps', 'configs', 'packages'],
  },
];

export default config;
