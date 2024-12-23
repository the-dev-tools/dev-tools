import { Config } from 'tailwindcss';
import animatePlugin from 'tailwindcss-animate';
import defaultTheme from 'tailwindcss/defaultTheme';

const alpha = (opacity: number) => Math.floor(255 * opacity).toString(16);

const config: Partial<Config> = {
  plugins: [
    // Disabled until this is fixed: https://github.com/adobe/react-spectrum/issues/5800
    // ariaPlugin({ prefix: 'rac' }),
    animatePlugin,
  ],
  theme: {
    extend: {
      fontFamily: {
        sans: ['"DM Sans Variable"', '"DM Sans"', ...defaultTheme.fontFamily.sans],
        mono: ['"DM Mono"', ...defaultTheme.fontFamily.mono],
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
      transitionProperty: {
        colors: [defaultTheme.transitionProperty.colors, 'outline-color', 'outline-width'].join(', '),
      },
    },
  },
};

export default config;
