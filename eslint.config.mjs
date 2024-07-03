import * as NodePath from 'path';
import * as NodeUrl from 'url';
import * as CompatUtils from '@eslint/compat';
import { FlatCompat } from '@eslint/eslintrc';
import JS from '@eslint/js';
import Prettier from 'eslint-config-prettier';
import ImportX from 'eslint-plugin-import-x';
import JsxA11y from 'eslint-plugin-jsx-a11y';
import ReactHooks from 'eslint-plugin-react-hooks';
import ReactJsxRuntime from 'eslint-plugin-react/configs/jsx-runtime.js';
import ReactRecommended from 'eslint-plugin-react/configs/recommended.js';
import Tailwind from 'eslint-plugin-tailwindcss';
import * as TS from 'typescript-eslint';

const filename = NodeUrl.fileURLToPath(import.meta.url);
const dirname = NodePath.dirname(filename);

const compat = new FlatCompat({
  baseDirectory: dirname,
  resolvePluginsRelativeTo: dirname,
});

const gitignore = CompatUtils.includeIgnoreFile(NodePath.resolve(dirname, '.gitignore'));

const commonjs = TS.config({
  files: ['postcss.config.js'],
  languageOptions: { sourceType: 'commonjs' },
});

const typescript = TS.config(
  {
    languageOptions: {
      parserOptions: {
        project: true,
        tsconfigRootDir: dirname,
      },
    },
  },
  ...TS.configs.strictTypeChecked,
  ...TS.configs.stylisticTypeChecked,
);

const imports = TS.config(
  {
    settings: {
      'import-x/parsers': { '@typescript-eslint/parser': ['.ts', '.tsx'] },
      'import-x/resolver': { typescript: true, node: true },
    },
  },
  ...compat.config(ImportX.configs.recommended),
  ImportX.configs.typescript,
);

const react = TS.config(
  { settings: { react: { version: 'detect' } } },
  {
    plugins: { 'react-hooks': CompatUtils.fixupPluginRules(ReactHooks) },
    rules: ReactHooks.configs.recommended.rules,
  },
  ReactRecommended,
  ReactJsxRuntime,
  JsxA11y.flatConfigs.recommended,
);

const rules = TS.config({
  rules: {
    '@typescript-eslint/no-confusing-void-expression': ['error', { ignoreVoidOperator: true }],
    '@typescript-eslint/no-meaningless-void-operator': 'off',
    '@typescript-eslint/no-misused-promises': ['error', { checksVoidReturn: false }],
    'import-x/namespace': 'off', // currently a lot of false-positives, re-enable if/when improved
    'react/prop-types': 'off',
  },
});

export default TS.config(
  gitignore,
  JS.configs.recommended,
  ...typescript,
  ...imports,
  ...react,
  ...Tailwind.configs['flat/recommended'],
  ...commonjs,
  Prettier,
  ...rules,
);
