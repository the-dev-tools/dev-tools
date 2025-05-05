import "@typespec/protobuf";
import "@typespec/rest";

import "../lib.tsp";

import "./body.tsp";
import "./endpoint.tsp";
import "./request.tsp";
import "./response.tsp";

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
@parentResource(Endpoint.Endpoint)
model Example {
  @key exampleId: Resource.Id;
  lastResponseId?: Resource.Id;
  name: string;
  @visibility(Resource.Mutation.None, Resource.Query.Get) breadcrumbs: string[];
  @visibility(Resource.Query.Get) bodyKind: Body.BodyKind;
}

model ExampleChange is Resource.Change<Example>;
model ExampleListItem is Resource.List.Item<Example>;
model ExampleListRequest is Resource.List.Request<Example>;
model ExampleListResponse is Resource.List.Response<Example, ExampleListItem>;
model ExampleVersionsItem is Resource.List.Item<Example> {
  ...ParentKeyOf<Example>;
}
model ExampleVersionsRequest is KeyOf<Example>;
model ExampleVersionsResponse is Resource.List.Response<Example, ExampleVersionsItem>;
model ExampleGetRequest is Resource.Get.Request<Example>;
model ExampleGetResponse is Resource.Get.Response<Example>;
model ExampleCreateRequest is Resource.Create.Request<Example>;
model ExampleCreateResponse is Resource.Create.Response<Example, ExampleListResponse>;
model ExampleUpdateRequest is Resource.Update.Request<Example>;
model ExampleUpdateResponse is Resource.Update.Response<Example>;
model ExampleDeleteRequest is Resource.Delete.Request<Example>;
model ExampleDeleteResponse is Resource.Delete.Response<Example>;

model ExampleDuplicateRequest is KeyOf<Example>;
model ExampleDuplicateResponse is Resource.Create.Response<Example, ExampleListResponse>;

model ExampleRunRequest is KeyOf<Example>;

@autoFields
model ExampleRunResponse is Resource.Get.Response<Response.Response> {
  ...Change.Changes;
}

@Protobuf.service
interface ExampleService {
  ExampleList(...ExampleListRequest): ExampleListResponse;
  ExampleVersions(...ExampleVersionsRequest): ExampleVersionsResponse;
  ExampleGet(...ExampleGetRequest): ExampleGetResponse;
  ExampleCreate(...ExampleCreateRequest): ExampleCreateResponse;
  ExampleUpdate(...ExampleUpdateRequest): ExampleUpdateResponse;
  ExampleDelete(...ExampleDeleteRequest): ExampleDeleteResponse;

  ExampleDuplicate(...ExampleDuplicateRequest): ExampleDuplicateResponse;

  ExampleRun(...ExampleRunRequest): ExampleRunResponse;
}
