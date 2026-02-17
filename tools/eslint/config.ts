import { includeIgnoreFile } from '@eslint/compat';
import js from '@eslint/js';
import tanStackRouter from '@tanstack/eslint-plugin-router';
import tsParser from '@typescript-eslint/parser';
import { Array, pipe, Record } from 'effect';
import { Linter } from 'eslint';
import prettier from 'eslint-config-prettier';
import tailwindPlugin from 'eslint-plugin-better-tailwindcss';
import { importX } from 'eslint-plugin-import-x';
import jsxA11y from 'eslint-plugin-jsx-a11y';
import perfectionistPlugin from 'eslint-plugin-perfectionist';
import reactPlugin from 'eslint-plugin-react';
import reactHooksPlugin from 'eslint-plugin-react-hooks';
import { defineConfig } from 'eslint/config';
import globals from 'globals';
import { resolve } from 'node:path';
import { configs as ts } from 'typescript-eslint';

const root = resolve(import.meta.dirname, '../..');

const gitignore = includeIgnoreFile(resolve(root, '.gitignore'));

const isIDE = process.env['NODE_ENV'] === 'IDE';

const nodejs = defineConfig({
  files: ['*.js', '*.mjs', '*.ts'],
  ignores: ['src/*'],
  languageOptions: { globals: { ...globals.node } },
});

const settings = defineConfig({
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
});

const react = defineConfig(
  jsxA11y.flatConfigs.recommended,
  reactPlugin.configs.flat['recommended']!,
  reactPlugin.configs.flat['jsx-runtime']!,
  reactHooksPlugin.configs.flat.recommended,
  {
    settings: {
      'react-hooks': {
        // https://tanstack.com/db/latest/docs/guides/live-queries#reactive-updates
        additionalEffectHooks: '(useLiveQuery|useLiveSuspenseQuery)',
      },
    },
  },
);

const tailwind = defineConfig({
  plugins: { 'better-tailwindcss': tailwindPlugin },
  rules: tailwindPlugin.configs['recommended']!.rules,
  settings: {
    'better-tailwindcss': {
      entryPoint: resolve(root, 'packages/ui/src/styles/index.css'),

      attributes: [],
      callees: [],
      tags: ['tw'],
      variables: [],
    },
  },
});

const perfectionist = defineConfig({
  plugins: { perfectionist: perfectionistPlugin },
  // Convert errors to warnings
  // eslint-disable-next-line import-x/no-named-as-default-member
  rules: Record.map(perfectionistPlugin.configs['recommended-natural'].rules ?? {}, (rule) => {
    if (!Array.isArray(rule)) return 'warn';
    return pipe(rule, Array.drop(1), Array.prepend('warn')) as ['warn', ...unknown[]];
  }),
  settings: {
    perfectionist: {
      partitionByNewLine: true,
    },
  },
});

const sortObject = (keys: string[], callingFunctionNamePattern?: string) => ({
  customGroups: Array.map(keys, (name) => ({ elementNamePattern: name, groupName: name })),
  groups: keys,
  useConfigurationIf: callingFunctionNamePattern ? { callingFunctionNamePattern } : { allNamesMatchPattern: keys },
});

const rules = defineConfig({
  rules: {
    'prefer-const': ['error', { destructuring: 'all' }],

    '@typescript-eslint/no-confusing-void-expression': ['error', { ignoreVoidOperator: true }],
    '@typescript-eslint/no-empty-object-type': ['error', { allowInterfaces: 'with-single-extends' }],
    '@typescript-eslint/no-meaningless-void-operator': 'off',
    '@typescript-eslint/no-misused-promises': ['error', { checksVoidReturn: false }],
    '@typescript-eslint/no-non-null-assertion': 'off', // in protobuf everything is optional, requiring assertions
    '@typescript-eslint/no-unnecessary-condition': ['error', { allowConstantLoopConditions: 'only-allowed-literals' }],
    '@typescript-eslint/no-unused-vars': [
      'error',
      { argsIgnorePattern: '^_', destructuredArrayIgnorePattern: '^_', varsIgnorePattern: '^_' },
    ],
    '@typescript-eslint/only-throw-error': [
      'error',
      {
        // https://tanstack.com/router/latest/docs/eslint/eslint-plugin-router#typescript-eslint
        allow: [{ from: 'package', name: 'Redirect', package: '@tanstack/router-core' }],
      },
    ],
    '@typescript-eslint/restrict-template-expressions': ['error', { allowNumber: true }],

    ...(isIDE && { 'import-x/no-unresolved': 'off' }), // disable in IDE due to false positives: https://github.com/un-ts/eslint-plugin-import-x/issues/370

    'perfectionist/sort-imports': [
      'warn',
      { internalPattern: ['^@the-dev-tools/.*', '^~.*'], newlinesBetween: 'ignore', newlinesInside: 'ignore' },
    ],
    'perfectionist/sort-modules': 'off', // consider re-enabling after https://github.com/azat-io/eslint-plugin-perfectionist/issues/434
    'perfectionist/sort-objects': [
      'warn',
      // Tailwind Variants function
      sortObject(['extend', 'base', 'slot', 'variants', 'defaultVariants', 'compoundVariants', 'compoundSlots'], 'tv'),
      sortObject(['xs', 'sm', 'md', 'lg', 'xl']),
      sortObject(['min', 'max']),
    ],

    'react/prop-types': 'off',

    'better-tailwindcss/enforce-consistent-line-wrapping': [
      'warn',
      { group: 'emptyLine', preferSingleLine: true, printWidth: 120 },
    ],
    'better-tailwindcss/enforce-consistent-variable-syntax': ['warn', { syntax: 'parentheses' }],
  },
});

// TODO: remove type castings when fixed upstream
// https://github.com/typescript-eslint/typescript-eslint/issues/11543
export default defineConfig(
  gitignore,
  settings,
  nodejs,

  prettier,

  perfectionist,

  js.configs.recommended,

  ts.strictTypeChecked,
  ts.stylisticTypeChecked,

  importX.flatConfigs.recommended as Linter.Config,
  importX.flatConfigs.typescript as Linter.Config,
  importX.flatConfigs.react as Linter.Config,

  react,

  tailwind,

  tanStackRouter.configs['flat/recommended'],

  rules,
);
