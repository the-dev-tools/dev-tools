using DevTools;

namespace API.Flow.Node;

model Position {
  x: float32;
  y: float32;
}

enum NodeKind {
  NoOp,
  Request,
  Condition,
  For,
  ForEach,
  Js,
}

@DataClient.entity
@parent(Flow)
model Node {
  @key nodeId: Resource.Id;
  kind: NodeKind;
  @visibility(T.Resource.Query.List) position: Position;
  @visibility(T.Resource.Query.List) noOp?: NodeNoOpKind;
  @visibility(T.Resource.Query.List, T.Resource.Mutation.None) state: NodeState;
  @visibility(T.Resource.Query.List, T.Resource.Mutation.None) info?: string;
  @visibility(T.Resource.Query.Get) name: string;
  @visibility(T.Resource.Query.Get) request?: NodeRequest;
  @visibility(T.Resource.Query.Get) condition?: NodeCondition;
  @visibility(T.Resource.Query.Get) for?: NodeFor;
  @visibility(T.Resource.Query.Get) forEach?: NodeForEach;
  @visibility(T.Resource.Query.Get) js?: NodeJS;
}

enum NodeNoOpKind {
  Start,
  Create,
  Then,
  Else,
  Loop,
}

model NodeRequest {
  collectionId: bytes;
  endpointId: bytes;
  exampleId: bytes;
  deltaEndpointId: bytes;
  deltaExampleId: bytes;
}

enum ErrorHandling {
  Ignore,
  Break,
}

model NodeFor {
  iterations: int32;
  condition: Condition.Condition;
  errorHandling: ErrorHandling;
}

model NodeForEach {
  path: string;
  condition: Condition.Condition;
  errorHandling: ErrorHandling;
}

model NodeCondition {
  condition: Condition.Condition;
}

model NodeJS {
  code: string;
}

model NodeListRequest is T.Resource.List.Request<Node>;
model NodeListResponse is T.Resource.List.Response<TRequest = NodeListRequest, TItem = NodeListItem>;
model NodeListItem is T.Resource.List.Item<Node>;

model NodeGetRequest is T.Resource.Get.Request<Node>;
model NodeGetResponse is T.Resource.Get.Response<Node>;

model NodeCreateRequest is T.Resource.Create.Request<Node>;
model NodeCreateResponse is T.Resource.Create.Response<Node>;

model NodeUpdateRequest is T.Resource.Update.Request<Node>;
model NodeUpdateResponse is T.Resource.Update.Response<Node>;

model NodeDeleteRequest is T.Resource.Delete.Request<Node>;
model NodeDeleteResponse is T.Resource.Delete.Response<Node>;

model NodeRunRequest {
  nodeId: Resource.Id;
  environmentId: Resource.Id;
}

model NodeRunResponse {
  state: NodeState;
}

enum NodeState {
  Running,
  Success,
  Failure,
  Canceled,
}

interface NodeService
  extends T.Resource.CRUD<
      TResource = Node,
      // List
      TListRequest = NodeListRequest,
      TListResponse = NodeListResponse,
      TListItem = NodeListItem,
      // Get
      TGetRequest = NodeGetRequest,
      TGetResponse = NodeGetResponse,
      // Create
      TCreateRequest = NodeCreateRequest,
      TCreateResponse = NodeCreateResponse,
      // Update
      TUpdateRequest = NodeUpdateRequest,
      TUpdateResponse = NodeUpdateResponse,
      // Delete
      TDeleteRequest = NodeDeleteRequest,
      TDeleteResponse = NodeDeleteResponse
    > {
  @Protobuf.stream(Protobuf.StreamMode.Out)
  NodeRun(...NodeRunRequest): NodeRunResponse;
}
