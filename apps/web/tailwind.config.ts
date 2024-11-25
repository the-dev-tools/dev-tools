import { type Config } from 'tailwindcss';

import { config } from '@the-dev-tools/config-tailwind';
import TailwindConfigCore from '@the-dev-tools/core/tailwind.config';

export default {
  content: [...TailwindConfigCore.content, './src/**/*.tsx'],
  presets: [config],
} satisfies Config;
