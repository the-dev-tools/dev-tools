import "@typespec/protobuf";
import "@typespec/rest";

import "./example.tsp";
import "./request.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "collection.item.response.v1",
  options: {
    go_package: "dev-tools-services/gen/collection/item/response/v1;responsev1",
  },
})
namespace API.CollectionItem.Response;

@parentResource(Example.Example)
model Response {
  @field(5) @key responseId: string;
  @field(6) status: int32;
  @field(7) body: bytes;
  @field(8) time: WellKnown.Timestamp;
  @field(9) duration: int32;
}

@parentResource(Response)
model Header {
  @field(6) @key headerId: string;
  @field(7) key: string;
  @field(8) value: string;
}

@parentResource(Response)
model Assert {
  @field(6) @key assertId: string;
  @field(7) asssert: Request.Assert;
  @field(8) value: string;
  @field(9) result: boolean;
}

model GetResponseRequest is Resource.Get.Request<Response>;
model GetResponseResponse is Resource.Get.Response<Response>;

model ListHeadersRequest is Resource.List.Request<Header>;
model ListHeadersResponse is Resource.List.Response<Header, Header>;

model ListAssertsRequest is Resource.List.Request<Assert>;
model ListAssertsResponse is Resource.List.Response<Assert, Assert>;

@Protobuf.service
interface CollectionItemResponseService {
  GetResponse(...GetResponseRequest): GetResponseResponse;

  ListHeaders(...ListHeadersRequest): ListHeadersResponse;

  ListAsserts(...ListAssertsRequest): ListAssertsResponse;
}
