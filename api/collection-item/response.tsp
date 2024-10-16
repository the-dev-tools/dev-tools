import "@typespec/protobuf";
import "@typespec/rest";

import "./example.tsp";
import "./request.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "collection.item.response.v1",
  options: {
    go_package: "dev-tools-spec/dist/buf/go/collection/item/response/v1;responsev1",
  },
})
namespace API.CollectionItem.Response;

@parentResource(Example.Example)
model Response {
  @field(5) @key responseId: Resource.ID;
  @field(6) status: int32;
  @field(7) body: bytes;
  @field(8) time: WellKnown.Timestamp;
  @field(9) duration: int32;
}

@parentResource(Response)
model ResponseHeader {
  @field(6) @key ResponseheaderId: Resource.ID;
  @field(7) key: string;
  @field(8) value: string;
}

@parentResource(Response)
model ResponseAssert {
  @field(6) asssert: Request.Assert;
  @field(7) result: boolean;
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
