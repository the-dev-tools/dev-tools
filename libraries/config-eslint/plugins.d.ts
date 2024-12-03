declare module 'eslint-plugin-react-hooks' {
  import { Linter, Rule } from 'eslint';
  const _default: {
    configs: { recommended: { rules: Linter.RulesRecord } };
    rules: Record<string, Rule.RuleModule>;
  };
  export default _default;
}
