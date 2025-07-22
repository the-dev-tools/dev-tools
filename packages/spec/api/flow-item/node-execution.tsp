import "@typespec/protobuf";
import "@typespec/rest";

import "../lib.tsp";
import "../resource.tsp";
import "../flow.tsp";
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

@parentResource(Flow.Flow)
@normalize
@entity
@autoFields
model NodeExecution {
  @key nodeExecutionId: Resource.Id;
  nodeId: Resource.Id;
  state: Node.NodeState;
  inputData?: bytes;
  outputData?: bytes;
  error?: string;
  
  // Execution timing
  @visibility(Resource.Query.List, Resource.Query.Get)
  startedAt?: WellKnown.Timestamp;
  @visibility(Resource.Query.List, Resource.Query.Get)
  completedAt?: WellKnown.Timestamp;
  @visibility(Resource.Query.List, Resource.Query.Get)
  durationMs?: int32;
}

@autoFields
model NodeExecutionListRequest is Resource.List.Request<NodeExecution> {
  // Filter by node ID
  nodeId?: Resource.Id;
  // Filter by execution state
  state?: Node.NodeState;
  // Filter by date range
  startedAfter?: WellKnown.Timestamp;
  startedBefore?: WellKnown.Timestamp;
}

model NodeExecutionListItem is Resource.List.Item<NodeExecution>;

@autoFields
model NodeExecutionGetRequest is Resource.Get.Request<NodeExecution>;

model NodeExecutionGetResponse is Resource.Get.Response<NodeExecution>;

@Protobuf.service
interface NodeExecutionService {
  // List executions by flow run or node
  NodeExecutionList is Resource.List.Operation<
    NodeExecution,
    TItem = NodeExecutionListItem,
    TRequest = NodeExecutionListRequest
  >;

  // Get specific execution details
  NodeExecutionGet is Resource.Get.Operation<
    NodeExecution,
    TRequest = NodeExecutionGetRequest,
    TResponse = NodeExecutionGetResponse
  >;
}