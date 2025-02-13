import "@typespec/protobuf";
import "@typespec/rest";

import "../../lib";

import "../collection.tsp";
import "../resource.tsp";
import "./example.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;
using TypeSpec.Rest.Resource;

@package({
  name: "collection.item.endpoint.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/collection/item/endpoint/v1;endpointv1",
  },
})
namespace API.CollectionItem.Endpoint;

@autoFields
@normalize
@parentResource(Collection.Collection)
model Endpoint {
  @key endpointId: Resource.Id;
  @visibility("query") @normalKey endpointVersionId?: Resource.Id;
  @visibility("list", "mutate") parentFolderId?: Resource.Id;
  name: string;
  method: string;
  @visibility("get", "mutate") url: string;
}

model EndpointChange is Resource.Change<Endpoint>;
model EndpointListItem is Resource.List.Item<Endpoint>;
model EndpointGetRequest is Resource.Get.Request<Endpoint> {
  @autoField @normalKey endpointVersionId?: Resource.Id;
}
model EndpointGetResponse is Resource.Get.Response<Endpoint>;
model EndpointCreateRequest is Resource.Create.Request<Endpoint>;
model EndpointCreateResponse is Resource.Create.Response<Endpoint> {
  ...KeyOf<Example.Example>;
}
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
