import "@typespec/protobuf";
import "@typespec/rest";

import "../lib";

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

enum ComparisonKind {
  COMPARISON_KIND_UNSPECIFIED: 0,
  COMPARISON_KIND_EQUAL: 1,
  COMPARISON_KIND_NOT_EQUAL: 2,
  COMPARISON_KIND_CONTAINS: 3,
  COMPARISON_KIND_NOT_CONTAINS: 4,
  COMPARISON_KIND_GREATER: 5,
  COMPARISON_KIND_LESS: 6,
  COMPARISON_KIND_GREATER_OR_EQUAL: 7,
  COMPARISON_KIND_LESS_OR_EQUAL: 8,
}

@autoFields
model Comparison {
  kind: ComparisonKind;
  left: string;
  right: string;
}
