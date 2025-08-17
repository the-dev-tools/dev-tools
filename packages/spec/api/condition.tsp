namespace API.Condition;

// TODO: add custom conditions
// enum ConditionKind {
//   Comparison: 1,
//   Custom: 2,
// }

model Condition {
  // kind: ConditionKind;
  comparison: Comparison;
  // custom?: string;
}

model Comparison {
  expression: string;
}
