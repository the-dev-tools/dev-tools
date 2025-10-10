namespace API.Condition;

// TODO: add custom conditions
// enum ConditionKind {
//   Unspecified,
//   Comparison,
//   Custom,
// }

model Condition {
  // kind: ConditionKind;
  comparison: Comparison;
  // custom?: string;
}

model Comparison {
  expression: string;
}
