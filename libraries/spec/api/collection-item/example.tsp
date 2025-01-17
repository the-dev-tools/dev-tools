import "@typespec/protobuf";
import "@typespec/rest";

import "../../lib";

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
  @visibility("get", "mutate") bodyKind: Body.BodyKind;
}

model ExampleChange is Resource.Change<Example>;
model ExampleListItem is Resource.List.Item<Example>;
model ExampleListRequest is Resource.List.Request<Example>;
model ExampleListResponse is Resource.List.Response<ExampleListItem>;
model ExampleGetRequest is Resource.Get.Request<Example>;
model ExampleGetResponse is Resource.Get.Response<Example>;
model ExampleCreateRequest is Resource.Create.Request<Example>;
model ExampleCreateResponse is Resource.Create.Response<Example>;
model ExampleUpdateRequest is Resource.Update.Request<Example>;
model ExampleUpdateResponse is Resource.Update.Response<Example>;
model ExampleDeleteRequest is Resource.Delete.Request<Example>;
model ExampleDeleteResponse is Resource.Delete.Response<Example>;

model ExampleDuplicateRequest is KeyOf<Example>;

model ExampleRunRequest is KeyOf<Example>;
model ExampleRunResponse is Resource.Get.Request<Response.Response>;

@Protobuf.service
interface ExampleService {
  ExampleList(...ExampleListRequest): ExampleListResponse;
  ExampleGet(...ExampleGetRequest): ExampleGetResponse;
  ExampleCreate(...ExampleCreateRequest): ExampleCreateResponse;
  ExampleUpdate(...ExampleUpdateRequest): ExampleUpdateResponse;
  ExampleDelete(...ExampleDeleteRequest): ExampleDeleteResponse;

  ExampleDuplicate(...ExampleDuplicateRequest): {};

  ExampleRun(...ExampleRunRequest): ExampleRunResponse;
}
