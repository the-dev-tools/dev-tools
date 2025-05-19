import "@typespec/protobuf";
import "@typespec/rest";

import "../lib.tsp";

import "./example.tsp";
import "./request.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "collection.item.response.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/collection/item/response/v1;responsev1",
  },
})
namespace API.CollectionItem.Response;

@autoFields
@normalize
@entity
@parentResource(Example.Example)
model Response {
  @key responseId: Resource.Id;
  status: int32;
  body: bytes;
  time: WellKnown.Timestamp;
  duration: int32;
  size: int32;
}

@autoFields
@normalize
@entity
@parentResource(Response)
model ResponseHeader {
  @key responseHeaderId: Resource.Id;
  key: string;
  value: string;
}

@autoFields
@parentResource(Response)
model ResponseAssert {
  assert: Request.Assert;
  result: boolean;
}

@Protobuf.service
interface ResponseService
  extends Resource.Get.Interface<Response>,
    Resource.List.Interface<ResponseHeader>,
    Resource.List.Interface<ResponseAssert> {}
