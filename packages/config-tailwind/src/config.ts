import * as Tailwind from 'tailwindcss';
import * as defaultTheme from 'tailwindcss/defaultTheme';

export const config: Omit<Tailwind.Config, 'content'> = {
  // eslint-disable-next-line @typescript-eslint/no-unsafe-call, @typescript-eslint/no-require-imports
  plugins: [require('tailwindcss-react-aria-components')({ prefix: 'rac' })],
  theme: {
    extend: {
      fontFamily: {
        sans: ['"Lexend Deca Variable"', '"Lexend Deca"', ...defaultTheme.fontFamily.sans],
      },
    },
  },
};
