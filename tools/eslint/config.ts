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
import perfectionistRaw from 'eslint-plugin-perfectionist';
import react from 'eslint-plugin-react';
// eslint-disable-next-line import-x/default
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

const reactHooks = defineConfig({
  extends: ['react-hooks/flat/recommended'],
  plugins: { 'react-hooks': reactHooksPlugin },
  // Opt-in additional rules
  // https://react.dev/reference/eslint-plugin-react-hooks#additional-rules
  rules: {
    'react-hooks/component-hook-factories': 'error',
    'react-hooks/config': 'error',
    'react-hooks/error-boundaries': 'error',
    'react-hooks/gating': 'error',
    'react-hooks/globals': 'error',
    'react-hooks/immutability': 'error',
    'react-hooks/incompatible-library': 'warn',
    'react-hooks/preserve-manual-memoization': 'error',
    'react-hooks/purity': 'error',
    'react-hooks/refs': 'error',
    'react-hooks/set-state-in-effect': 'error',
    'react-hooks/set-state-in-render': 'error',
    'react-hooks/static-components': 'error',
    'react-hooks/unsupported-syntax': 'error',
    'react-hooks/use-memo': 'error',
  },
});

const tailwind = defineConfig({
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
});

const perfectionist = defineConfig({
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
});

const sortObject = (keys: string[], callingFunctionNamePattern?: string) => ({
  customGroups: Array.map(keys, (name) => ({ elementNamePattern: name, groupName: name })),
  groups: keys,
  useConfigurationIf: callingFunctionNamePattern ? { callingFunctionNamePattern } : { allNamesMatchPattern: keys },
});

const rules = defineConfig({
  rules: {
    '@typescript-eslint/no-confusing-void-expression': ['error', { ignoreVoidOperator: true }],
    '@typescript-eslint/no-empty-object-type': ['error', { allowInterfaces: 'with-single-extends' }],
    '@typescript-eslint/no-meaningless-void-operator': 'off',
    '@typescript-eslint/no-misused-promises': ['error', { checksVoidReturn: false }],
    '@typescript-eslint/no-non-null-assertion': 'off', // in protobuf everything is optional, requiring assertions
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
      { internalPattern: ['^@the-dev-tools/.*', '^~.*'], newlinesBetween: 'ignore' },
    ],
    'perfectionist/sort-modules': 'off', // consider re-enabling after https://github.com/azat-io/eslint-plugin-perfectionist/issues/434
    'perfectionist/sort-objects': [
      'warn',
      // Tailwind Variants function
      sortObject(['extend', 'base', 'slot', 'variants', 'defaultVariants', 'compoundVariants', 'compoundSlots'], 'tv'),
      sortObject(['sm', 'md', 'lg', 'xl']),
      sortObject(['min', 'max']),
    ],

    'react-hooks/exhaustive-deps': [
      'warn',
      {
        // https://dataclient.io/docs/api/useLoading#eslint
        additionalHooks: '(useLoading)',
      },
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

  react.configs.flat['recommended']!,
  react.configs.flat['jsx-runtime']!,
  reactHooks,

  jsxA11y.flatConfigs.recommended,

  tailwind,

  tanStackRouter.configs['flat/recommended'],

  rules,
);
