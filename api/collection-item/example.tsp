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
    go_package: "dev-tools-services/gen/collection/item/example/v1;examplev1",
  },
})
namespace API.CollectionItem.Example;

@parentResource(Api.Api)
model Example {
  @field(4) @key exampleId: Resource.ID;
  @field(5) lastResponseId?: string;
  @field(6) name: string;
  @field(7) @visibility("get") bodyKind: Request.BodyKind;
}

model ListExampleItem is Resource.List.Item<Example>;
model GetExampleRequest is Resource.Get.Request<Example>;
model GetExampleResponse is Resource.Get.Response<Example>;
model CreateExampleRequest is Resource.Create.Request<Example>;
model CreateExampleResponse is Resource.Create.Response<Example>;
model UpdateExampleRequest is Resource.Update.Request<Example>;
model UpdateExampleResponse is Resource.Update.Response<Example>;
model DeleteExampleRequest is Resource.Delete.Request<Example>;
model DeleteExampleResponse is Resource.Delete.Response<Example>;

model DupliacteExampleRequest is KeyOf<Example>;

model RunExampleRequest is KeyOf<Example>;
model RunExampleResponse is Resource.Get.Request<Response.Response>;

@Protobuf.service
interface CollectionItemExampleService {
  GetExample(...GetExampleRequest): GetExampleResponse;
  CreateExample(...CreateExampleRequest): CreateExampleResponse;
  UpdateExample(...UpdateExampleRequest): UpdateExampleResponse;
  DeleteExample(...DeleteExampleRequest): DeleteExampleResponse;

  DupliacteExample(...DupliacteExampleRequest): {};

  RunExample(...RunExampleRequest): RunExampleResponse;
}
