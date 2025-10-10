using DevTools;

namespace API.Collection.Item.Request;

@DataClient.entity
@parent(Example.Example)
model Query {
  @key queryId: Resource.Id;
  key: string;
  enabled: boolean;
  value: string;
  description: string;
}

model QueryMoveRequest {
  ...ParentKeyOf<Query>;
  ...KeyOf<Query>;
  position: Resource.MovePosition;
  targetQueryId: Resource.Id;
}

model QueryListRequest is T.Resource.List.Request<Query>;
model QueryListResponse is T.Resource.List.Response<TRequest = QueryListRequest, TItem = QueryListItem>;
model QueryListItem is T.Resource.List.Item<Query>;

model QueryCreateRequest is T.Resource.Create.Request<Query>;
model QueryCreateResponse is T.Resource.Create.Response<Query>;

model QueryUpdateRequest is T.Resource.Update.Request<Query>;
model QueryUpdateResponse is T.Resource.Update.Response<Query>;

model QueryDeleteRequest is T.Resource.Delete.Request<Query>;
model QueryDeleteResponse is T.Resource.Delete.Response<Query>;

model QueryDelta is T.Delta.Item<Query>;

model QueryDeltaListRequest is T.Delta.List.Request<QueryDelta>;
model QueryDeltaListResponse is T.Delta.List.Response<TRequest = QueryDeltaListRequest, TItem = QueryDeltaListItem>;
model QueryDeltaListItem is T.Delta.List.Item<QueryDelta>;

model QueryDeltaCreateRequest is T.Delta.Create.Request<QueryDelta>;
model QueryDeltaCreateResponse is T.Delta.Create.Response<QueryDelta>;

model QueryDeltaUpdateRequest is T.Delta.Update.Request<QueryDelta>;
model QueryDeltaUpdateResponse is T.Delta.Update.Response<QueryDelta>;

model QueryDeltaDeleteRequest is T.Delta.Delete.Request<QueryDelta>;
model QueryDeltaDeleteResponse is T.Delta.Delete.Response<QueryDelta>;

model QueryDeltaResetRequest is T.Delta.Reset.Request<QueryDelta>;
model QueryDeltaResetResponse is T.Delta.Reset.Response<QueryDelta>;

model QueryDeltaMoveRequest {
  ...ParentKeyOf<QueryDelta>;
  ...KeyOf<QueryDelta>;
  originId: Resource.Id;
  position: Resource.MovePosition;
  targetQueryId: Resource.Id;
}

@DataClient.entity
@parent(Example.Example)
model Header {
  @key headerId: Resource.Id;
  key: string;
  enabled: boolean;
  value: string;
  description: string;
}

model HeaderListRequest is T.Resource.List.Request<Header>;
model HeaderListResponse is T.Resource.List.Response<TRequest = HeaderListRequest, TItem = HeaderListItem>;
model HeaderListItem is T.Resource.List.Item<Header>;

model HeaderCreateRequest is T.Resource.Create.Request<Header>;
model HeaderCreateResponse is T.Resource.Create.Response<Header>;

model HeaderUpdateRequest is T.Resource.Update.Request<Header>;
model HeaderUpdateResponse is T.Resource.Update.Response<Header>;

model HeaderDeleteRequest is T.Resource.Delete.Request<Header>;
model HeaderDeleteResponse is T.Resource.Delete.Response<Header>;

model HeaderMoveRequest {
  ...ParentKeyOf<Header>;
  ...KeyOf<Header>;
  position: Resource.MovePosition;
  targetHeaderId: Resource.Id;
}

model HeaderDelta is T.Delta.Item<Header>;

model HeaderDeltaListRequest is T.Delta.List.Request<HeaderDelta>;
model HeaderDeltaListResponse is T.Delta.List.Response<TRequest = HeaderDeltaListRequest, TItem = HeaderDeltaListItem>;
model HeaderDeltaListItem is T.Delta.List.Item<HeaderDelta>;

model HeaderDeltaCreateRequest is T.Delta.Create.Request<HeaderDelta>;
model HeaderDeltaCreateResponse is T.Delta.Create.Response<HeaderDelta>;

model HeaderDeltaUpdateRequest is T.Delta.Update.Request<HeaderDelta>;
model HeaderDeltaUpdateResponse is T.Delta.Update.Response<HeaderDelta>;

model HeaderDeltaDeleteRequest is T.Delta.Delete.Request<HeaderDelta>;
model HeaderDeltaDeleteResponse is T.Delta.Delete.Response<HeaderDelta>;

model HeaderDeltaResetRequest is T.Delta.Reset.Request<HeaderDelta>;
model HeaderDeltaResetResponse is T.Delta.Reset.Response<HeaderDelta>;

model HeaderDeltaMoveRequest {
  ...ParentKeyOf<HeaderDelta>;
  ...KeyOf<HeaderDelta>;
  originId: Resource.Id;
  position: Resource.MovePosition;
  targetHeaderId: Resource.Id;
}

@DataClient.entity
@parent(Example.Example)
model Assert {
  @key assertId: Resource.Id;
  condition: Condition.Condition;
}

model AssertListRequest is T.Resource.List.Request<Assert>;
model AssertListResponse is T.Resource.List.Response<TRequest = AssertListRequest, TItem = AssertListItem>;
model AssertListItem is T.Resource.List.Item<Assert>;

model AssertGetRequest is T.Resource.Get.Request<Assert>;
model AssertGetResponse is T.Resource.Get.Response<Assert>;

model AssertCreateRequest is T.Resource.Create.Request<Assert>;
model AssertCreateResponse is T.Resource.Create.Response<Assert>;

model AssertUpdateRequest is T.Resource.Update.Request<Assert>;
model AssertUpdateResponse is T.Resource.Update.Response<Assert>;

model AssertDeleteRequest is T.Resource.Delete.Request<Assert>;
model AssertDeleteResponse is T.Resource.Delete.Response<Assert>;

model AssertDelta is T.Delta.Item<Assert>;

model AssertDeltaListRequest is T.Delta.List.Request<AssertDelta>;
model AssertDeltaListResponse is T.Delta.List.Response<TRequest = AssertDeltaListRequest, TItem = AssertDeltaListItem>;
model AssertDeltaListItem is T.Delta.List.Item<AssertDelta>;

model AssertDeltaCreateRequest is T.Delta.Create.Request<AssertDelta>;
model AssertDeltaCreateResponse is T.Delta.Create.Response<AssertDelta>;

model AssertDeltaUpdateRequest is T.Delta.Update.Request<AssertDelta>;
model AssertDeltaUpdateResponse is T.Delta.Update.Response<AssertDelta>;

model AssertDeltaDeleteRequest is T.Delta.Delete.Request<AssertDelta>;
model AssertDeltaDeleteResponse is T.Delta.Delete.Response<AssertDelta>;

model AssertDeltaResetRequest is T.Delta.Reset.Request<AssertDelta>;
model AssertDeltaResetResponse is T.Delta.Reset.Response<AssertDelta>;

interface RequestService
  extends BlankInterface,
    // Query
    T.Resource.List.Interface<
      TResource = Query,
      TRequest = QueryListRequest,
      TResponse = QueryListResponse,
      TItem = QueryListItem
    >,
    T.Resource.Create.Interface<
      TResource = Query,
      TRequest = QueryCreateRequest,
      TResponse = QueryCreateResponse,
      TListRequest = QueryListRequest,
      TListItem = QueryListItem
    >,
    T.Resource.Update.Interface<TResource = Query, TRequest = QueryUpdateRequest, TResponse = QueryUpdateResponse>,
    T.Resource.Delete.Interface<TResource = Query, TRequest = QueryDeleteRequest, TResponse = QueryDeleteResponse>,
    T.Delta.Interface<
      TResource = QueryDelta,
      // List
      TListRequest = QueryDeltaListRequest,
      TListResponse = QueryDeltaListResponse,
      TListItem = QueryDeltaListItem,
      // Create
      TCreateRequest = QueryDeltaCreateRequest,
      TCreateResponse = QueryDeltaCreateResponse,
      // Update
      TUpdateRequest = QueryDeltaUpdateRequest,
      TUpdateResponse = QueryDeltaUpdateResponse,
      // Delete
      TDeleteRequest = QueryDeltaDeleteRequest,
      TDeleteResponse = QueryDeltaDeleteResponse,
      // Reset
      TResetRequest = QueryDeltaResetRequest,
      TResetResponse = QueryDeltaResetResponse
    >,
    // Header
    T.Resource.List.Interface<
      TResource = Header,
      TRequest = HeaderListRequest,
      TResponse = HeaderListResponse,
      TItem = HeaderListItem
    >,
    T.Resource.Create.Interface<
      TResource = Header,
      TRequest = HeaderCreateRequest,
      TResponse = HeaderCreateResponse,
      TListRequest = HeaderListRequest,
      TListItem = HeaderListItem
    >,
    T.Resource.Update.Interface<TResource = Header, TRequest = HeaderUpdateRequest, TResponse = HeaderUpdateResponse>,
    T.Resource.Delete.Interface<TResource = Header, TRequest = HeaderDeleteRequest, TResponse = HeaderDeleteResponse>,
    T.Delta.Interface<
      TResource = HeaderDelta,
      // List
      TListRequest = HeaderDeltaListRequest,
      TListResponse = HeaderDeltaListResponse,
      TListItem = HeaderDeltaListItem,
      // Create
      TCreateRequest = HeaderDeltaCreateRequest,
      TCreateResponse = HeaderDeltaCreateResponse,
      // Update
      TUpdateRequest = HeaderDeltaUpdateRequest,
      TUpdateResponse = HeaderDeltaUpdateResponse,
      // Delete
      TDeleteRequest = HeaderDeltaDeleteRequest,
      TDeleteResponse = HeaderDeltaDeleteResponse,
      // Reset
      TResetRequest = HeaderDeltaResetRequest,
      TResetResponse = HeaderDeltaResetResponse
    >,
    // Assert
    T.Resource.List.Interface<
      TResource = Assert,
      TRequest = AssertListRequest,
      TResponse = AssertListResponse,
      TItem = AssertListItem
    >,
    T.Resource.Create.Interface<
      TResource = Assert,
      TRequest = AssertCreateRequest,
      TResponse = AssertCreateResponse,
      TListRequest = AssertListRequest,
      TListItem = AssertListItem
    >,
    T.Resource.Update.Interface<TResource = Assert, TRequest = AssertUpdateRequest, TResponse = AssertUpdateResponse>,
    T.Resource.Delete.Interface<TResource = Assert, TRequest = AssertDeleteRequest, TResponse = AssertDeleteResponse>,
    T.Delta.Interface<
      TResource = AssertDelta,
      // List
      TListRequest = AssertDeltaListRequest,
      TListResponse = AssertDeltaListResponse,
      TListItem = AssertDeltaListItem,
      // Create
      TCreateRequest = AssertDeltaCreateRequest,
      TCreateResponse = AssertDeltaCreateResponse,
      // Update
      TUpdateRequest = AssertDeltaUpdateRequest,
      TUpdateResponse = AssertDeltaUpdateResponse,
      // Delete
      TDeleteRequest = AssertDeltaDeleteRequest,
      TDeleteResponse = AssertDeltaDeleteResponse,
      // Reset
      TResetRequest = AssertDeltaResetRequest,
      TResetResponse = AssertDeltaResetResponse
    > {
  @DataClient.endpoint("request.js:moveQuery")
  QueryMove(...QueryMoveRequest): {};
  @DataClient.endpoint("request.js:moveQueryDelta")
  QueryDeltaMove(...QueryDeltaMoveRequest): {};
  @DataClient.endpoint("request.js:moveHeader")
  HeaderMove(...HeaderMoveRequest): {};
  @DataClient.endpoint("request.js:moveHeaderDelta")
  HeaderDeltaMove(...HeaderDeltaMoveRequest): {};
}
