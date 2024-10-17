declare module 'eslint-plugin-react-hooks' {
  import { Linter, Rule } from 'eslint';
  export default {
    configs: { recommended: { rules: {} as Linter.RulesRecord } },
    rules: {} as Record<string, Rule.OldStyleRule>,
  };
}
