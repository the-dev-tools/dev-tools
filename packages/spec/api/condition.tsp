import "@typespec/protobuf";
import "@typespec/rest";

import "./lib.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "condition.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/condition/v1;conditionv1",
  },
})
namespace API.Condition;

// TODO: add custom conditions
// enum ConditionKind {
//   CONDITION_KIND_UNSPECIFIED: 0,
//   CONDITION_KIND_COMPARISON: 1,
//   CONDITION_KIND_CUSTOM: 2,
// }

@autoFields
model Condition {
  // kind: ConditionKind;
  comparison: Comparison;
  // custom?: string;
}

@autoFields
model Comparison {
  expression: string;
}
