import * as Tailwind from 'tailwindcss';
import * as defaultTheme from 'tailwindcss/defaultTheme';

const config: Tailwind.Config = {
  content: ['./src/**/*.tsx'],
  // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
  plugins: [
    // eslint-disable-next-line @typescript-eslint/no-unsafe-call, @typescript-eslint/no-var-requires
    require('tailwindcss-react-aria-components')({ prefix: 'rac' }),
  ],
  theme: {
    extend: {
      fontFamily: {
        sans: ['"Lexend Deca Variable"', '"Lexend Deca"', ...defaultTheme.fontFamily.sans],
      },
    },
  },
};

export default config;
