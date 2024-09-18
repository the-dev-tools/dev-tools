import * as NodePath from 'path';
import * as NodeUrl from 'url';
import * as CompatUtils from '@eslint/compat';
import JS from '@eslint/js';
import Prettier from 'eslint-config-prettier';
import ImportX from 'eslint-plugin-import-x';
import JsxA11y from 'eslint-plugin-jsx-a11y';
import ReactHooks from 'eslint-plugin-react-hooks';
import ReactJsxRuntime from 'eslint-plugin-react/configs/jsx-runtime.js';
import ReactRecommended from 'eslint-plugin-react/configs/recommended.js';
import Tailwind from 'eslint-plugin-tailwindcss';
import Globals from 'globals';
import * as TS from 'typescript-eslint';

const gitignore = CompatUtils.includeIgnoreFile(
  NodePath.resolve(NodePath.dirname(NodeUrl.fileURLToPath(import.meta.url)), '../../.gitignore'),
);

const commonjs = TS.config({
  files: ['postcss.config.js'],
  languageOptions: { sourceType: 'commonjs' },
});

const nodejs = TS.config({
  files: ['*.js', '*.mjs', '*.ts'],
  ignores: ['src/*'],
  languageOptions: { globals: { ...Globals.node } },
});

const typescript = TS.config(...TS.configs.strictTypeChecked, ...TS.configs.stylisticTypeChecked, {
  languageOptions: {
    parserOptions: {
      projectService: true,
      tsconfigRootDir: process.cwd(),
    },
  },
});

const imports = TS.config(ImportX.flatConfigs.recommended, ImportX.flatConfigs.typescript, {
  settings: {
    'import-x/parsers': { '@typescript-eslint/parser': ['.ts', '.tsx'] },
    'import-x/resolver': { typescript: true, node: true },
  },
});

const tailwind = TS.config(...Tailwind.configs['flat/recommended'], {
  settings: {
    tailwindcss: {
      callees: ['tv', 'twMerge', 'twJoin'],
      tags: ['tw'],
    },
  },
});

const rules = TS.config({
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
});

export const eslintBaseConfig = TS.config(
  gitignore,
  JS.configs.recommended,
  ...typescript,
  ...imports,
  ...tailwind,
  ...commonjs,
  ...nodejs,
  Prettier,
  ...rules,
);

export const eslintReactConfig = TS.config(
  ...eslintBaseConfig,
  ImportX.flatConfigs.react,
  { settings: { react: { version: 'detect' } } },
  {
    plugins: { 'react-hooks': CompatUtils.fixupPluginRules(ReactHooks) },
    rules: ReactHooks.configs.recommended.rules,
  },
  ReactRecommended,
  ReactJsxRuntime,
  JsxA11y.flatConfigs.recommended,
  {
    rules: {
      'react/prop-types': 'off',
    },
  },
);
