import "@typespec/protobuf";
import "@typespec/rest";

import "../lib.tsp";

import "../collection.tsp";
import "./folder.tsp";
import "./endpoint.tsp";
import "./example.tsp";

import "./body.tsp";
import "./request.tsp";
import "./response.tsp";
// import "./breadcrumb.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;
using TypeSpec.Rest.Resource;

@package({
  name: "collection.item.example.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/collection/item/example/v1;examplev1",
  },
})
namespace API.CollectionItem.Example;

@autoFields
@normalize
@entity
@parentResource(Endpoint.Endpoint)
model Example {
  @key exampleId: Resource.Id;
  lastResponseId?: Resource.Id;
  name: string;
  @visibility(Resource.Mutation.None, Resource.Query.Get) breadcrumbs: ExampleBreadcrumb[];
  @visibility(Resource.Query.Get) bodyKind: Body.BodyKind;
}

enum ExampleBreadcrumbKind {
  EXAMPLE_BREADCRUMB_KIND_UNSPECIFIED: 0,
  EXAMPLE_BREADCRUMB_KIND_COLLECTION: 1,
  EXAMPLE_BREADCRUMB_KIND_FOLDER: 2,
  EXAMPLE_BREADCRUMB_KIND_ENDPOINT: 3,
}

@autoFields
model ExampleBreadcrumb {
  kind: ExampleBreadcrumbKind;
  collection?: Collection.CollectionListItem;
  folder?: Folder.FolderListItem;
  endpoint?: Endpoint.EndpointListItem;
}

model ExampleListItem is Resource.List.Item<Example>;
model ExampleListResponse is Resource.List.Response<Example, TItem = ExampleListItem>;
model ExampleVersionsItem is Resource.List.Item<Example> {
  ...ParentKeyOf<Example>;
}
model ExampleVersionsRequest is KeyOf<Example>;
model ExampleVersionsResponse
  is Resource.List.Response<Example, TRequest = ExampleVersionsRequest, TItem = ExampleVersionsItem>;

model ExampleDuplicateRequest {
  ...KeyOf<Example>;
  ...ParentKeyOf<Example>;
}
model ExampleDuplicateResponse is Resource.Create.Response<Example>;

model ExampleRunRequest is KeyOf<Example>;

@autoFields
model ExampleRunResponse {
  response: Resource.Get.Response<Response.Response>;
  version: ExampleVersionsItem;
}

@Protobuf.service
interface ExampleService
  extends Resource.CRUD<Example, TListItem = ExampleListItem, TListResponse = ExampleListResponse> {
  ExampleVersions is Resource.List.Operation<
    Example,
    TItem = ExampleVersionsItem,
    TRequest = ExampleVersionsRequest,
    TResponse = ExampleVersionsResponse
  >;

  ExampleDuplicate is Resource.Create.Operation<
    Example,
    TRequest = ExampleDuplicateRequest,
    TResponse = ExampleDuplicateResponse,
    TListItem = ExampleListItem
  >;

  ExampleRun(...ExampleRunRequest): ExampleRunResponse;
}
