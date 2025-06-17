/**
 * @see https://prettier.io/docs/en/options
 * @type { import('prettier').Options }
 */
export default {
  overrides: [{ files: '*.tsp', options: { parser: 'typespec' } }],

  plugins: ['@typespec/prettier-plugin-typespec'],

  // Quotes
  jsxSingleQuote: true,
  singleQuote: true,
};
