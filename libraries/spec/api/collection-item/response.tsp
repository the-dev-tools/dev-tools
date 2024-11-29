import "@typespec/protobuf";
import "@typespec/rest";

import "../../lib";

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
@parentResource(Example.Example)
model Response {
  @key responseId: Resource.ID;
  status: int32;
  body: bytes;
  time: WellKnown.Timestamp;
  duration: int32;
}

@autoFields
@parentResource(Response)
model ResponseHeader {
  @key ResponseheaderId: Resource.ID;
  key: string;
  value: string;
}

@autoFields
@parentResource(Response)
model ResponseAssert {
  assert: Request.Assert;
  result: boolean;
}

model ResponseGetRequest is Resource.Get.Request<Response>;
model ResponseGetResponse is Resource.Get.Response<Response>;

model ResponseHeaderListItem is Resource.List.Item<ResponseHeader>;
model ResponseHeaderListRequest is Resource.List.Request<ResponseHeader>;
model ResponseHeaderListResponse is Resource.List.Response<ResponseHeaderListItem>;

model ResponseAssertListItem is Resource.List.Item<ResponseAssert>;
model ResponseAssertListRequest is Resource.List.Request<ResponseAssert>;
model ResponseAssertListResponse is Resource.List.Response<ResponseAssertListItem>;

@Protobuf.service
interface ResponseService {
  ResponseGet(...ResponseGetRequest): ResponseGetResponse;
  ResponseHeaderList(...ResponseHeaderListRequest): ResponseHeaderListResponse;
  ResponseAssertList(...ResponseAssertListRequest): ResponseAssertListResponse;
}
