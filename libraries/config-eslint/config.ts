import { resolve } from 'node:path';
import { includeIgnoreFile } from '@eslint/compat';
import js from '@eslint/js';
import tsParser from '@typescript-eslint/parser';
import { Linter } from 'eslint';
import prettier from 'eslint-config-prettier';
import { flatConfigs as importX } from 'eslint-plugin-import-x';
import jsxA11y from 'eslint-plugin-jsx-a11y';
import react from 'eslint-plugin-react';
import reactHooksOld from 'eslint-plugin-react-hooks';
import tailwind from 'eslint-plugin-tailwindcss';
import globals from 'globals';
import { ConfigArray, configs as ts } from 'typescript-eslint';

const gitignore = includeIgnoreFile(resolve(import.meta.dirname, '../../.gitignore'));

const commonjs: Linter.Config = {
  files: ['postcss.config.js'],
  languageOptions: { sourceType: 'commonjs' },
};

const nodejs: Linter.Config = {
  files: ['*.js', '*.mjs', '*.ts'],
  ignores: ['src/*'],
  languageOptions: { globals: { ...globals.node } },
};

const settings: Linter.Config = {
  settings: {
    react: { version: 'detect' },
    tailwindcss: {
      // This might not be needed after this PR is merged
      // https://github.com/francoismassart/eslint-plugin-tailwindcss/pull/380
      config: resolve(import.meta.dirname, '../config-tailwind/src/config.ts'),
      callees: ['tv', 'twMerge', 'twJoin'],
      tags: ['tw'],
    },
  },
  languageOptions: {
    parser: tsParser,
    globals: globals.browser,
    parserOptions: {
      projectService: true,
      tsconfigRootDir: process.cwd(),
    },
  },
};

const reactHooks = {
  plugins: { 'react-hooks': reactHooksOld },
  rules: reactHooksOld.configs.recommended.rules,
};

const rules: Linter.Config = {
  rules: {
    '@typescript-eslint/no-confusing-void-expression': ['error', { ignoreVoidOperator: true }],
    '@typescript-eslint/no-empty-object-type': ['error', { allowInterfaces: 'with-single-extends' }],
    '@typescript-eslint/no-meaningless-void-operator': 'off',
    '@typescript-eslint/no-misused-promises': ['error', { checksVoidReturn: false }],
    '@typescript-eslint/no-non-null-assertion': 'off', // in protobuf everything is optional, requiring assertions
    '@typescript-eslint/no-unused-vars': ['error', { destructuredArrayIgnorePattern: '^_' }],
    '@typescript-eslint/restrict-template-expressions': ['error', { allowNumber: true }],
    'import-x/namespace': 'off', // currently a lot of false-positives, re-enable if/when improved
    'react/prop-types': 'off',
  },
};

const config: ConfigArray = [
  gitignore,
  settings,
  commonjs,
  nodejs,

  prettier,

  js.configs.recommended,

  ...ts.strictTypeChecked,
  ...ts.stylisticTypeChecked,

  importX.recommended,
  importX.typescript,
  importX.react,

  react.configs.flat['recommended']!,
  react.configs.flat['jsx-runtime']!,
  reactHooks,

  jsxA11y.flatConfigs.recommended,

  ...tailwind.configs['flat/recommended'],

  rules,
];

export default config;
