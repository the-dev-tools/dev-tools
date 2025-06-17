import { includeIgnoreFile } from '@eslint/compat';
import js from '@eslint/js';
import tsParser from '@typescript-eslint/parser';
import { Array, pipe, Record } from 'effect';
import { Linter } from 'eslint';
import prettier from 'eslint-config-prettier';
import tailwindPlugin from 'eslint-plugin-better-tailwindcss';
import { flatConfigs as importX } from 'eslint-plugin-import-x';
import jsxA11y from 'eslint-plugin-jsx-a11y';
import perfectionistRaw from 'eslint-plugin-perfectionist';
import react from 'eslint-plugin-react';
import * as reactCompilerPlugin from 'eslint-plugin-react-compiler';
import { configs as reactHooks } from 'eslint-plugin-react-hooks';
import globals from 'globals';
import { resolve } from 'node:path';
import { ConfigArray, configs as ts } from 'typescript-eslint';

const root = resolve(import.meta.dirname, '../..');

const gitignore = includeIgnoreFile(resolve(root, '.gitignore'));

const nodejs: Linter.Config = {
  files: ['*.js', '*.mjs', '*.ts'],
  ignores: ['src/*'],
  languageOptions: { globals: { ...globals.node } },
};

const settings: Linter.Config = {
  languageOptions: {
    globals: globals.browser,
    parser: tsParser,
    parserOptions: {
      projectService: true,
      tsconfigRootDir: process.cwd(),
    },
  },
  settings: {
    react: { version: 'detect' },
  },
};

const reactCompiler: Linter.Config = {
  plugins: { 'react-compiler': reactCompilerPlugin },
  rules: { 'react-compiler/react-compiler': 'error' },
};

const tailwind: Linter.Config = {
  plugins: { 'better-tailwindcss': tailwindPlugin },
  rules: tailwindPlugin.configs['recommended']!.rules,
  settings: {
    'better-tailwindcss': {
      entryPoint: resolve(root, 'packages/ui/src/styles.css'),

      attributes: [],
      callees: [],
      tags: ['tw'],
      variables: [],
    },
  },
};

const perfectionist = {
  plugins: { perfectionist: perfectionistRaw },
  // Convert errors to warnings
  rules: Record.map(perfectionistRaw.configs['recommended-natural'].rules ?? {}, (rule) => {
    if (!Array.isArray(rule)) return 'warn';
    return pipe(rule, Array.drop(1), Array.prepend('warn'));
  }),
  settings: {
    perfectionist: {
      partitionByNewLine: true,
    },
  },
};

// Implement TanStack Router rule via Perfectionist
// https://tanstack.com/router/latest/docs/eslint/create-route-property-order
// https://perfectionist.dev/rules/sort-objects#useconfigurationif
const sortRouterObject = pipe(
  [['params', 'validateSearch'], ['loaderDeps', 'search'], ['context'], ['beforeLoad'], ['loader']],
  (groups) => ({
    customGroups: Array.map(groups, (names, index) => ({
      elementNamePattern: names,
      groupName: String(index),
    })),
    groups: Array.map(groups, (_, index) => String(index)),
    useConfigurationIf: { callingFunctionNamePattern: 'makeRoute' },
  }),
);

// Consistent Tailwind Variants order
const sortTVObject = pipe(
  ['extend', 'base', 'slot', 'variants', 'defaultVariants', 'compoundVariants', 'compoundSlots'],
  (groups) => ({
    customGroups: Array.map(groups, (name) => ({
      elementNamePattern: name,
      groupName: name,
    })),
    groups,
    useConfigurationIf: { callingFunctionNamePattern: 'tv' },
  }),
);

const rules: Linter.Config = {
  rules: {
    '@typescript-eslint/no-confusing-void-expression': ['error', { ignoreVoidOperator: true }],
    '@typescript-eslint/no-empty-object-type': ['error', { allowInterfaces: 'with-single-extends' }],
    '@typescript-eslint/no-meaningless-void-operator': 'off',
    '@typescript-eslint/no-misused-promises': ['error', { checksVoidReturn: false }],
    '@typescript-eslint/no-non-null-assertion': 'off', // in protobuf everything is optional, requiring assertions
    '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_', destructuredArrayIgnorePattern: '^_' }],
    '@typescript-eslint/restrict-template-expressions': ['error', { allowNumber: true }],

    'import-x/namespace': 'off', // currently a lot of false-positives, re-enable if/when improved

    'perfectionist/sort-imports': [
      'warn',
      { internalPattern: ['^@the-dev-tools/.*', '^~.*'], newlinesBetween: 'ignore' },
    ],
    'perfectionist/sort-modules': 'off', // consider re-enabling after https://github.com/azat-io/eslint-plugin-perfectionist/issues/434
    'perfectionist/sort-objects': ['warn', sortRouterObject, sortTVObject],

    'react-hooks/exhaustive-deps': [
      'warn',
      {
        // https://dataclient.io/docs/api/useLoading#eslint
        additionalHooks: '(useLoading)',
      },
    ],

    'react/prop-types': 'off',

    'better-tailwindcss/enforce-consistent-variable-syntax': ['warn', { syntax: 'parentheses' }],
    'better-tailwindcss/multiline': ['warn', { group: 'emptyLine', preferSingleLine: true, printWidth: 120 }],
  },
};

const config: ConfigArray = [
  gitignore,
  settings,
  nodejs,

  prettier,

  perfectionist,

  js.configs.recommended,

  ...ts.strictTypeChecked,
  ...ts.stylisticTypeChecked,

  importX.recommended,
  importX.typescript,
  importX.react,

  react.configs.flat['recommended']!,
  react.configs.flat['jsx-runtime']!,
  reactHooks['recommended-latest'],
  reactCompiler,

  jsxA11y.flatConfigs.recommended,

  tailwind,

  rules,
];

export default config;
