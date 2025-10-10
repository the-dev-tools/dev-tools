using DevTools;

namespace API.Collection.Item.Example;

@DataClient.entity
@parent(Endpoint.Endpoint)
model Example {
  @key exampleId: Resource.Id;
  lastResponseId?: Resource.Id;
  name: string;
  @visibility(T.Resource.Mutation.Create, T.Resource.Query.List) hidden?: boolean;
  @visibility(T.Resource.Mutation.None, T.Resource.Query.Get) breadcrumbs: ExampleBreadcrumb[];
  @visibility(T.Resource.Query.Get) bodyKind: Body.BodyKind;
  @visibility(T.Resource.Query.Get) queryCount: int32;
  @visibility(T.Resource.Query.Get) headerCount: int32;
  @visibility(T.Resource.Query.Get) bodyCount: int32;
  @visibility(T.Resource.Query.Get) assertCount: int32;
}

enum ExampleBreadcrumbKind {
  Collection,
  Folder,
  Endpoint,
}

model ExampleBreadcrumb {
  kind: ExampleBreadcrumbKind;
  collection?: CollectionListItem;
  folder?: Folder.FolderListItem;
  endpoint?: Endpoint.EndpointListItem;
}

model ExampleListRequest is T.Resource.List.Request<Example>;
model ExampleListResponse is T.Resource.List.Response<TRequest = ExampleListRequest, TItem = ExampleListItem>;
model ExampleListItem is T.Resource.List.Item<Example>;

model ExampleGetRequest is T.Resource.Get.Request<Example>;
model ExampleGetResponse is T.Resource.Get.Response<Example>;

model ExampleCreateRequest is T.Resource.Create.Request<Example>;
model ExampleCreateResponse is T.Resource.Create.Response<Example>;

model ExampleUpdateRequest is T.Resource.Update.Request<Example>;
model ExampleUpdateResponse is T.Resource.Update.Response<Example>;

model ExampleDeleteRequest is T.Resource.Delete.Request<Example>;
model ExampleDeleteResponse is T.Resource.Delete.Response<Example>;

model ExampleVersionListRequest is KeyOf<Example>;
model ExampleVersionListResponse
  is T.Resource.List.Response<TRequest = ExampleVersionListRequest, TItem = ExampleVersionListItem>;
model ExampleVersionListItem is T.Resource.List.Item<Example>;

model ExampleDuplicateRequest {
  ...KeyOf<Example>;
  ...ParentKeyOf<Example>;
}
model ExampleDuplicateResponse is T.Resource.Create.Response<Example>;

model ExampleRunRequest is KeyOf<Example>;

model ExampleRunResponse {
  response: Response.ResponseGetResponse;
  version: ExampleVersionListItem;
}

model ExampleMoveRequest {
  ...ParentKeyOf<Example>;
  ...KeyOf<Example>;
  position: Resource.MovePosition;
  targetExampleId: Resource.Id;
}

model ExampleMoveResponse {}

interface ExampleService
  extends T.Resource.CRUD<
      TResource = Example,
      // List
      TListRequest = ExampleListRequest,
      TListResponse = ExampleListResponse,
      TListItem = ExampleListItem,
      // Get
      TGetRequest = ExampleGetRequest,
      TGetResponse = ExampleGetResponse,
      // Create
      TCreateRequest = ExampleCreateRequest,
      TCreateResponse = ExampleCreateResponse,
      // Update
      TUpdateRequest = ExampleUpdateRequest,
      TUpdateResponse = ExampleUpdateResponse,
      // Delete
      TDeleteRequest = ExampleDeleteRequest,
      TDeleteResponse = ExampleDeleteResponse
    > {
  ExampleVersionList is T.Resource.List.Operation<
    TRequest = ExampleVersionListRequest,
    TResponse = ExampleVersionListResponse,
    TItem = ExampleVersionListItem
  >;

  ExampleDuplicate is T.Resource.Create.Operation<
    TRequest = ExampleDuplicateRequest,
    TResponse = ExampleDuplicateResponse,
    TListRequest = ExampleListRequest,
    TListItem = ExampleListItem
  >;

  @DataClient.endpoint("collection-item.js:runExample")
  ExampleRun(...ExampleRunRequest): ExampleRunResponse;

  @DataClient.endpoint("collection-item.js:moveExample")
  ExampleMove(...ExampleMoveRequest): ExampleMoveResponse;
}
