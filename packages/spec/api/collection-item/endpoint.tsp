using DevTools;

namespace API.CollectionItem.Endpoint;

@DataClient.entity
@parent(Collection.Collection)
model Endpoint {
  @key endpointId: Resource.Id;
  @visibility(Resource.Query.List) parentFolderId?: Resource.Id;
  @visibility(Resource.Mutation.Create, Resource.Query.List) hidden?: boolean;
  name: string;
  method: string;
  @visibility(Resource.Query.Get) url: string;
}

@instanceOf(Resource.Create.CreateResponse<Endpoint>)
model EndpointCreateResponse {
  ...KeyOf<Endpoint>;
  ...KeyOf<Example.Example>;
}

model EndpointDuplicateRequest {
  ...Resource.Create.CreateRequest<Endpoint>;
}

model EndpointDuplicateResponse {
  ...Resource.Create.CreateResponse<Endpoint>;
}

interface EndpointService
  extends Resource.Get.Interface<Endpoint>,
    Resource.Update.Interface<Endpoint>,
    Resource.Delete.Interface<Endpoint> {
  @DataClient.endpoint("collection-item.js:createEndpoint")
  EndpointCreate(...Resource.Create.CreateRequest<Endpoint>): EndpointCreateResponse;
  @DataClient.endpoint("collection-item.js:createEndpoint")
  EndpointDuplicate(...EndpointDuplicateRequest): EndpointDuplicateResponse;
}
