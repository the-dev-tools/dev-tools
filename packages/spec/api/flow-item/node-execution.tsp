using DevTools;

namespace API.Flow.Node.Execution;

@DataClient.entity
@parent(Node)
model NodeExecution {
  @key nodeExecutionId: Resource.Id;
  nodeId: Resource.Id;
  name: string;
  state: NodeState;
  error?: string;
  input?: Protobuf.WellKnown.Json;
  output?: Protobuf.WellKnown.Json;
  responseId?: Resource.Id; // present when execution belongs to a request node
  completedAt?: Protobuf.WellKnown.Timestamp;
}

model NodeExecutionListRequest is T.Resource.List.Request<NodeExecution>;
model NodeExecutionListResponse
  is T.Resource.List.Response<TRequest = NodeExecutionListRequest, TItem = NodeExecutionListItem>;
model NodeExecutionListItem is T.Resource.List.Item<NodeExecution>;

model NodeExecutionGetRequest is T.Resource.Get.Request<NodeExecution>;
model NodeExecutionGetResponse is T.Resource.Get.Response<NodeExecution>;

interface NodeExecutionService
  extends T.Resource.List.Interface<
      TResource = NodeExecution,
      TRequest = NodeExecutionListRequest,
      TResponse = NodeExecutionListResponse,
      TItem = NodeExecutionListItem
    >,
    T.Resource.Get.Interface<
      TResource = NodeExecution,
      TRequest = NodeExecutionGetRequest,
      TResponse = NodeExecutionGetResponse
    > {}
