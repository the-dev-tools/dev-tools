import { Config } from 'tailwindcss';
import animatePlugin from 'tailwindcss-animate';
import ariaPlugin from 'tailwindcss-react-aria-components';
import * as defaultTheme from 'tailwindcss/defaultTheme';

const alpha = (opacity: number) => Math.floor(255 * opacity).toString(16);

export const config: Partial<Config> = {
  plugins: [ariaPlugin({ prefix: 'rac' }), animatePlugin],
  theme: {
    extend: {
      fontFamily: {
        sans: ['"Lexend Deca Variable"', '"Lexend Deca"', ...defaultTheme.fontFamily.sans],
      },
      fontSize: {
        md: '0.8125rem',
      },
      borderRadius: {
        ms: '0.3125rem',
      },
      boxShadow: (_) => ({
        search: [
          `2px 0 60px 28px ${_.theme('colors.slate.400')}${alpha(0.1)}`,
          `1px 25px 56px -4px ${_.theme('colors.slate.500')}${alpha(0.2)}`,
        ].join(', '),
      }),
    },
  },
};
