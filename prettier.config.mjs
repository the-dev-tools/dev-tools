/**
 * @see https://prettier.io/docs/en/options
 * @type {import('prettier').Options}
 */
export default {
  singleQuote: true,
  jsxSingleQuote: true,

  plugins: ['@ianvs/prettier-plugin-sort-imports'],

  /**
   * @see https://github.com/IanVS/prettier-plugin-sort-imports?tab=readme-ov-file#importorder
   * @type {import('@ianvs/prettier-plugin-sort-imports').PluginConfig['importOrder']}
   */
  importOrder: [
    '<BUILTIN_MODULES>',
    '<THIRD_PARTY_MODULES>',
    '',
    '^@plasmo/(.*)$',
    '',
    '^@plasmohq/(.*)$',
    '',
    '^@the-dev-tools/(.*)$',
    '',
    '^~(.*)$',
    '',
    '^[./]',
  ],
};
