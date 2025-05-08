import { Linter } from 'eslint';

import defaultConfig from '@the-dev-tools/eslint-config';

const rules: Linter.RulesRecord = {
  'react/jsx-key': 'off',
};

const config: typeof defaultConfig = [...defaultConfig, { rules }];

export default config;
