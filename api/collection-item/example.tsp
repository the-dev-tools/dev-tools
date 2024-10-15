import "@typespec/protobuf";
import "@typespec/rest";

import "./api.tsp";
import "./request.tsp";
import "./response.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;
using TypeSpec.Rest.Resource;

@package({
  name: "collection.item.example.v1",
  options: {
    go_package: "dev-tools-spec/dist/buf/go/collection/item/example/v1;examplev1",
  },
})
namespace API.CollectionItem.Example;

@parentResource(Api.Api)
model Example {
  @field(4) @key exampleId: Resource.ID;
  @field(5) lastResponseId?: Resource.ID;
  @field(6) name: string;
  @field(7) @visibility("get", "mutate") bodyKind: Request.BodyKind;
}

model ListExamplesItem is Resource.List.Item<Example>;
model ListExamplesRequest is Resource.List.Request<Example>;
model ListExamplesResponse is Resource.List.Response<ListExamplesItem>;
model GetExampleRequest is Resource.Get.Request<Example>;
model GetExampleResponse is Resource.Get.Response<Example>;
model CreateExampleRequest is Resource.Create.Request<Example>;
model CreateExampleResponse is Resource.Create.Response<Example>;
model UpdateExampleRequest is Resource.Update.Request<Example>;
model UpdateExampleResponse is Resource.Update.Response<Example>;
model DeleteExampleRequest is Resource.Delete.Request<Example>;
model DeleteExampleResponse is Resource.Delete.Response<Example>;

model DuplicateExampleRequest is KeyOf<Example>;

model RunExampleRequest is KeyOf<Example>;
model RunExampleResponse is Resource.Get.Request<Response.Response>;

@Protobuf.service
interface CollectionItemExampleService {
  ListExamples(...ListExamplesRequest): ListExamplesResponse;
  GetExample(...GetExampleRequest): GetExampleResponse;
  CreateExample(...CreateExampleRequest): CreateExampleResponse;
  UpdateExample(...UpdateExampleRequest): UpdateExampleResponse;
  DeleteExample(...DeleteExampleRequest): DeleteExampleResponse;

  DuplicateExample(...DuplicateExampleRequest): {};

  RunExample(...RunExampleRequest): RunExampleResponse;
}
