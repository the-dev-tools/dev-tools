import * as Tailwind from 'tailwindcss';
import animatePlugin from 'tailwindcss-animate';
import ariaPlugin from 'tailwindcss-react-aria-components';
import * as defaultTheme from 'tailwindcss/defaultTheme';

export const config: Omit<Tailwind.Config, 'content'> = {
  plugins: [ariaPlugin({ prefix: 'rac' }), animatePlugin],
  theme: {
    extend: {
      fontFamily: {
        sans: ['"Lexend Deca Variable"', '"Lexend Deca"', ...defaultTheme.fontFamily.sans],
      },
    },
  },
};
