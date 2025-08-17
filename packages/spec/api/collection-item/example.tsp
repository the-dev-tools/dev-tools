using DevTools;

namespace API.CollectionItem.Example;

@DataClient.entity
@parent(Endpoint.Endpoint)
model Example {
  @key exampleId: Resource.Id;
  lastResponseId?: Resource.Id;
  name: string;
  @visibility(Resource.Mutation.Create, Resource.Query.List) hidden?: boolean;
  @visibility(Resource.Mutation.None, Resource.Query.Get) breadcrumbs: ExampleBreadcrumb[];
  @visibility(Resource.Query.Get) bodyKind: Body.BodyKind;
  @visibility(Resource.Query.Get) queryCount: int32;
  @visibility(Resource.Query.Get) headerCount: int32;
  @visibility(Resource.Query.Get) bodyCount: int32;
  @visibility(Resource.Query.Get) assertCount: int32;
}

enum ExampleBreadcrumbKind {
  Collection,
  Folder,
  Endpoint,
}

model ExampleBreadcrumb {
  kind: ExampleBreadcrumbKind;
  collection?: Resource.List.ListItem<Collection.Collection>;
  folder?: Resource.List.ListItem<Folder.Folder>;
  endpoint?: Resource.List.ListItem<Endpoint.Endpoint>;
}

model ExampleVersion is Example;

@instanceOf(Resource.List.ListRequest<ExampleVersion>)
model ExampleVersionListRequest is KeyOf<ExampleVersion>;

model ExampleDuplicateRequest {
  ...KeyOf<Example>;
  ...ParentKeyOf<Example>;
}

model ExampleDuplicateResponse {
  ...Resource.Create.CreateResponse<Example>;
}

model ExampleRunRequest is KeyOf<Example>;

model ExampleRunResponse {
  response: Resource.Get.GetResponse<Response.Response>;
  version: ExampleVersion;
}

model ExampleMoveRequest {
  ...ParentKeyOf<Example>;
  ...KeyOf<Example>;
  position: Resource.MovePosition;
  targetExampleId: Resource.Id;
}

model ExampleMoveResponse {}

interface ExampleService extends Resource.CRUD<Example>, Resource.List.Interface<ExampleVersion> {
  ExampleDuplicate is Resource.Create.Operation<
    Example,
    TRequest = ExampleDuplicateRequest,
    TResponse = ExampleDuplicateResponse
  >;

  @DataClient.endpoint("collection-item.js:runExample")
  ExampleRun(...ExampleRunRequest): ExampleRunResponse;

  @DataClient.endpoint("collection-item.js:moveExample")
  ExampleMove(...ExampleMoveRequest): ExampleMoveResponse;
}
