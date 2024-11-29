/**
 * @see https://prettier.io/docs/en/options
 * @type {import('prettier').Options}
 */
export default {
  singleQuote: true,
  jsxSingleQuote: true,
  plugins: ['@typespec/prettier-plugin-typespec'],
  overrides: [{ files: '*.tsp', options: { parser: 'typespec' } }],
};
