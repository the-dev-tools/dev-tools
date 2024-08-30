declare module 'eslint-plugin-react/configs/recommended.js' {
  import { TSESLint } from '@typescript-eslint/utils';
  declare const config: TSESLint.FlatConfig.Config;
  export default config;
}

declare module 'eslint-plugin-react/configs/jsx-runtime.js' {
  import { TSESLint } from '@typescript-eslint/utils';
  declare const config: TSESLint.FlatConfig.Config;
  export default config;
}

declare module 'eslint-plugin-react-hooks' {
  import { Linter, Rule } from 'eslint';
  export const configs: { recommended: { rules: Linter.RulesRecord } };
  export const rules: Record<string, Rule.OldStyleRule>;
}
