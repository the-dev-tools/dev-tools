using DevTools;

namespace API.Flow.Edge;

enum Handle {
  Then,
  Else,
  Loop,
}

enum EdgeKind {
  NoOp,
}

@DataClient.entity
@parent(Flow)
model Edge {
  @key edgeId: Resource.Id;
  kind: EdgeKind;
  sourceId: Resource.Id;
  targetId: Resource.Id;
  sourceHandle: Handle;
}

model EdgeListRequest is T.Resource.List.Request<Edge>;
model EdgeListResponse is T.Resource.List.Response<TRequest = EdgeListRequest, TItem = EdgeListItem>;
model EdgeListItem is T.Resource.List.Item<Edge>;

model EdgeGetRequest is T.Resource.Get.Request<Edge>;
model EdgeGetResponse is T.Resource.Get.Response<Edge>;

model EdgeCreateRequest is T.Resource.Create.Request<Edge>;
model EdgeCreateResponse is T.Resource.Create.Response<Edge>;

model EdgeUpdateRequest is T.Resource.Update.Request<Edge>;
model EdgeUpdateResponse is T.Resource.Update.Response<Edge>;

model EdgeDeleteRequest is T.Resource.Delete.Request<Edge>;
model EdgeDeleteResponse is T.Resource.Delete.Response<Edge>;

interface EdgeService
  extends T.Resource.CRUD<
      TResource = Edge,
      // List
      TListRequest = EdgeListRequest,
      TListResponse = EdgeListResponse,
      TListItem = EdgeListItem,
      // Get
      TGetRequest = EdgeGetRequest,
      TGetResponse = EdgeGetResponse,
      // Create
      TCreateRequest = EdgeCreateRequest,
      TCreateResponse = EdgeCreateResponse,
      // Update
      TUpdateRequest = EdgeUpdateRequest,
      TUpdateResponse = EdgeUpdateResponse,
      // Delete
      TDeleteRequest = EdgeDeleteRequest,
      TDeleteResponse = EdgeDeleteResponse
    > {}
