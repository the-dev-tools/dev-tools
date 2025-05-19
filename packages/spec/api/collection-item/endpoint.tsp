import "@typespec/protobuf";
import "@typespec/rest";

import "../lib.tsp";

import "../collection.tsp";
import "../resource.tsp";
import "./example.tsp";
import "./item.tsp";

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
@entity
@parentResource(Collection.Collection)
model Endpoint {
  @key endpointId: Resource.Id;
  @visibility(Resource.Query.List) parentFolderId?: Resource.Id;
  name: string;
  method: string;
  @visibility(Resource.Query.Get) url: string;
}

model EndpointChange is Resource.Change<Endpoint>;
model EndpointListItem is Resource.List.Item<Endpoint>;
model EndpointCreateRequest is Resource.Create.Request<Endpoint>;
model EndpointCreateResponse is KeyOf<Endpoint> {
  ...KeyOf<Example.Example>;
}

model EndpointDuplicateRequest is EndpointCreateRequest;
model EndpointDuplicateResponse is EndpointCreateResponse;

@Protobuf.service
interface EndpointService
  extends Resource.Get.Interface<Endpoint>,
    Resource.Update.Interface<Endpoint>,
    Resource.Delete.Interface<Endpoint> {
  @endpoint("collection-item.js:createEndpoint")
  EndpointCreate(...EndpointCreateRequest): EndpointCreateResponse;
  @endpoint("collection-item.js:createEndpoint")
  EndpointDuplicate(...EndpointDuplicateRequest): EndpointDuplicateResponse;
}
