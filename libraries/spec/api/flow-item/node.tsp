import "@typespec/protobuf";
import "@typespec/rest";

import "../../lib";

import "../resource.tsp";
import "../flow.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "flow.node.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/flow/node/v1;nodev1",
  },
})
namespace API.FlowItem.Node;

enum NodeKind {
  NODE_KIND_UNSPECIFIED: 0,
  NODE_KIND_FOR: 1,
  NODE_KIND_REQUEST: 2,
  NODE_KIND_CONDITION: 3,
}

// TODO: replace with union when supported
@parentResource(Flow.Flow)
@autoFields
model Node {
  kind: NodeKind;
  for?: NodeFor;
  request?: NodeRequest;
  condition?: NodeCondition;
}

@autoFields
model NodeFor {
  @key NodeId: Resource.ID;
  nextNodeId?: Resource.ID;
  iteration: int32;
}

@autoFields
model NodeRequest {
  @key nodeID: Resource.ID;
  nextNodeID?: Resource.ID;
  exampleID: bytes;
}

enum ConditionType {
  CONDITION_TYPE_UNSPECIFIED: 0,
  CONDITION_TYPE_EQUAL: 1,
  CONDITION_TYPE_NOT_EQUAL: 2,
  CONDITION_TYPE_CONTAINS: 3,
  CONDITION_TYPE_NOT_CONTAINS: 4,
  CONDITION_TYPE_GREATER: 5,
  CONDITION_TYPE_ASSERT_TYPE_LESS: 6,
  CONDITION_TYPE_GREATER_OR_EQUAL: 7,
  CONDITION_TYPE_LESS_OR_EQUAL: 8,
  CONDITION_TYPE_EXISTS: 9,
  CONDITION_TYPE_NOT_EXISTS: 10,
  CONDITION_TYPE_CUSTOM: 11,
}

@autoFields
model NodeCondition {
  @key nodeId: Resource.ID;
  nextNodeId?: Resource.ID;
  conditionType: ConditionType;
  customCondition?: string;
}
