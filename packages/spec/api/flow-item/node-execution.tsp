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

@parentResource(Node.Node)
@entity
@autoFields
model NodeExecution {
  @key nodeExecutionId: Resource.Id;
  nodeId: Resource.Id;
  name: string;
  state: Node.NodeState;
  error?: string;
  input?: WellKnown.JsonValue;
  output?: WellKnown.JsonValue;
  responseId?: Resource.Id; // present when execution belongs to a request node
  completedAt?: WellKnown.Timestamp;
}

@Protobuf.service
interface NodeExecutionService extends Resource.List.Interface<NodeExecution>, Resource.Get.Interface<NodeExecution> {}
