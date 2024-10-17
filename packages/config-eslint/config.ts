import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { includeIgnoreFile } from '@eslint/compat';
import js from '@eslint/js';
import tsParser from '@typescript-eslint/parser';
import { Linter } from 'eslint';
import prettier from 'eslint-config-prettier';
import { flatConfigs as importX } from 'eslint-plugin-import-x';
import jsxA11y from 'eslint-plugin-jsx-a11y';
import react from 'eslint-plugin-react';
import reactHooks from 'eslint-plugin-react-hooks';
import tailwind from 'eslint-plugin-tailwindcss';
import globals from 'globals';
import { config, configs as ts } from 'typescript-eslint';

const gitignore = includeIgnoreFile(resolve(dirname(fileURLToPath(import.meta.url)), '../../.gitignore'));

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
  },
};

export const eslintBaseConfig = config(
  gitignore,
  js.configs.recommended,
  ...ts.strictTypeChecked,
  ...ts.stylisticTypeChecked,
  importX.recommended,
  importX.typescript,
  ...tailwind.configs['flat/recommended'],
  commonjs,
  nodejs,
  prettier,
  settings,
  rules,
);

export const eslintReactConfig = config(
  ...eslintBaseConfig,
  importX.react,
  { settings: { react: { version: 'detect' } } },
  // eslint-disable-next-line import-x/no-named-as-default-member
  react.configs.flat.recommended as Linter.Config,
  // eslint-disable-next-line import-x/no-named-as-default-member
  react.configs.flat['jsx-runtime'] as Linter.Config,
  {
    plugins: { 'react-hooks': reactHooks },
    rules: reactHooks.configs.recommended.rules,
  },
  jsxA11y.flatConfigs.recommended,
  { rules: { 'react/prop-types': 'off' } },
);
