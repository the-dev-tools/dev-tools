import { type Config } from 'tailwindcss';

import { config } from '@the-dev-tools/config-tailwind';

export default {
  content: [`${__dirname}/src/**/*.tsx`, 'cosmos.decorator.tsx', './cosmos/**/*.tsx'],
  presets: [config],
} satisfies Config;
