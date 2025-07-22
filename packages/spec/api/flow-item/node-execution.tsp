import "@typespec/protobuf";
import "@typespec/rest";

import "../lib.tsp";
import "../resource.tsp";
import "../well-known.tsp";

import "./node.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "flow.node.execution.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/flow/node/execution/v1;nodeexecutionv1",
  },
})
namespace API.FlowItem.NodeExecution;

// OutputKind enum for UI representation
enum OutputKind {
  OUTPUT_KIND_UNSPECIFIED: 0,
  OUTPUT_KIND_REQUEST: 1,
}

@parentResource(Node.Node)
@entity
@autoFields
model NodeExecution {
  @key nodeExecutionId: Resource.Id;
  nodeId: Resource.Id;
  state: Node.NodeState;
  error?: string;
  input?: WellKnown.JsonValue;
  output?: WellKnown.JsonValue;
  outputKind?: OutputKind; // OutputKind for UI representation
  completedAt?: WellKnown.Timestamp;
}

@Protobuf.service
interface NodeExecutionService
  extends Resource.List.Interface<NodeExecution>,
    Resource.Get.Interface<NodeExecution> {}
