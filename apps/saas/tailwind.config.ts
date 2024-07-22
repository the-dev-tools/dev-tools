import { type Config } from 'tailwindcss';

import { config } from '@the-dev-tools/config-tailwind';
import { tailwindContent } from '@the-dev-tools/ui/tailwind-content';

export default {
  content: ['./src/**/*.tsx', ...tailwindContent],
  presets: [config],
} satisfies Config;
