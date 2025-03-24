/**
 * @see https://prettier.io/docs/en/options
 * @see https://github.com/tailwindlabs/prettier-plugin-tailwindcss?tab=readme-ov-file#options
 * @see https://github.com/IanVS/prettier-plugin-sort-imports?tab=readme-ov-file#importorder
 * @type { import('prettier').Options | import('prettier-plugin-tailwindcss').PluginOptions | import('@ianvs/prettier-plugin-sort-imports').PluginConfig }
 */
export default {
  singleQuote: true,
  jsxSingleQuote: true,

  plugins: [
    '@ianvs/prettier-plugin-sort-imports',
    '@typespec/prettier-plugin-typespec',
    // ! Replace with `eslint-plugin-tailwindcss` once Tailwind 4 is supported
    // https://github.com/francoismassart/eslint-plugin-tailwindcss/issues/325
    'prettier-plugin-tailwindcss',
  ],

  overrides: [{ files: '*.tsp', options: { parser: 'typespec' } }],

  tailwindStylesheet: './libraries/ui/src/styles.css',
  tailwindFunctions: ['tw'],

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
