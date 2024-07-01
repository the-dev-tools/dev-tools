import * as Tailwind from 'tailwindcss';
import * as defaultTheme from 'tailwindcss/defaultTheme';

const config: Tailwind.Config = {
  content: ['./src/**/*.tsx'],
  theme: {
    extend: {
      fontFamily: {
        sans: ['"Lexend Deca Variable"', ...defaultTheme.fontFamily.sans],
      },
    },
  },
};

export default config;
