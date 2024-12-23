import { type Config } from 'tailwindcss';

import config from '@the-dev-tools/config-tailwind';
import TailwindConfigUI from '@the-dev-tools/ui/tailwind.config';

export default {
  content: [...TailwindConfigUI.content, `${__dirname}/src/**/*.tsx`],
  presets: [config],
} satisfies Config;
