import { type Config } from 'tailwindcss';

import { config } from '@the-dev-tools/config-tailwind';

import { tailwindContent } from './src/tailwind-content.mjs';

export default {
  content: tailwindContent,
  presets: [config],
} satisfies Config;
