/**
 * @see https://prettier.io/docs/en/options
 * @see https://github.com/tailwindlabs/prettier-plugin-tailwindcss?tab=readme-ov-file#options
 * @type { import('prettier').Options | import('prettier-plugin-tailwindcss').PluginOptions }
 */
export default {
  overrides: [{ files: '*.tsp', options: { parser: 'typespec' } }],

  plugins: [
    '@typespec/prettier-plugin-typespec',
    // ! Replace with `eslint-plugin-tailwindcss` once Tailwind 4 is supported
    // https://github.com/francoismassart/eslint-plugin-tailwindcss/issues/325
    'prettier-plugin-tailwindcss',
  ],

  // Quotes
  jsxSingleQuote: true,
  singleQuote: true,

  // Tailwind
  tailwindFunctions: ['tw'],
  tailwindStylesheet: './packages/ui/src/styles.css',
};
