import "@typespec/protobuf";
import "@typespec/rest";

import "../collection.tsp";
import "../resource.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;
using TypeSpec.Rest.Resource;

@package({
  name: "collection.item.endpoint.v1",
  options: {
    go_package: "dev-tools-spec/dist/buf/go/collection/item/endpoint/v1;endpointv1",
  },
})
namespace API.CollectionItem.Endpoint;

@parentResource(Collection.Collection)
model Endpoint {
  @field(3) @key endpointId: Resource.ID;
  @field(4) @visibility("list", "mutate") parentFolderId?: Resource.ID;
  @field(5) name: string;
  @field(6) method: string;
  @field(7) @visibility("get", "mutate") url: string;
}

model EndpointListItem is Resource.List.Item<Endpoint>;
model EndpointGetRequest is Resource.Get.Request<Endpoint>;
model EndpointGetResponse is Resource.Get.Response<Endpoint>;
model EndpointCreateRequest is Resource.Create.Request<Endpoint>;
model EndpointCreateResponse is Resource.Create.Response<Endpoint>;
model EndpointUpdateRequest is Resource.Update.Request<Endpoint>;
model EndpointUpdateResponse is Resource.Update.Response<Endpoint>;
model EndpointDeleteRequest is Resource.Delete.Request<Endpoint>;
model EndpointDeleteResponse is Resource.Delete.Response<Endpoint>;

model EndpointDuplicateRequest is KeyOf<Endpoint>;

@Protobuf.service
interface EndpointService {
  EndpointGet(...EndpointGetRequest): EndpointGetResponse;
  EndpointCreate(...EndpointCreateRequest): EndpointCreateResponse;
  EndpointUpdate(...EndpointUpdateRequest): EndpointUpdateResponse;
  EndpointDelete(...EndpointDeleteRequest): EndpointDeleteResponse;

  EndpointDuplicate(...EndpointDuplicateRequest): {};
}
