using DevTools;

namespace API.Collection.Item.Endpoint;

@DataClient.entity
@parent(Collection)
model Endpoint {
  @key endpointId: Resource.Id;
  @visibility(T.Resource.Query.List) parentFolderId?: Resource.Id;
  @visibility(T.Resource.Mutation.Create, T.Resource.Query.List) hidden?: boolean;
  name: string;
  method: string;
  @visibility(T.Resource.Query.Get) url: string;
}

model EndpointGetRequest is T.Resource.Get.Request<Endpoint>;
model EndpointGetResponse is T.Resource.Get.Response<Endpoint>;

model EndpointUpdateRequest is T.Resource.Update.Request<Endpoint>;
model EndpointUpdateResponse is T.Resource.Update.Response<Endpoint>;

model EndpointDeleteRequest is T.Resource.Delete.Request<Endpoint>;
model EndpointDeleteResponse is T.Resource.Delete.Response<Endpoint>;

model EndpointCreateRequest is T.Resource.Create.Request<Endpoint>;
model EndpointCreateResponse is KeyOf<Endpoint> {
  ...KeyOf<Example.Example>;
}
model EndpointListItem is T.Resource.List.Item<Endpoint>;

model EndpointDuplicateRequest is EndpointCreateRequest;
model EndpointDuplicateResponse is EndpointCreateResponse;

interface EndpointService
  extends T.Resource.Get.Interface<
      TResource = Endpoint,
      TRequest = EndpointGetRequest,
      TResponse = EndpointGetResponse
    >,
    T.Resource.Update.Interface<
      TResource = Endpoint,
      TRequest = EndpointUpdateRequest,
      TResponse = EndpointUpdateResponse
    >,
    T.Resource.Delete.Interface<
      TResource = Endpoint,
      TRequest = EndpointDeleteRequest,
      TResponse = EndpointDeleteResponse
    > {
  @DataClient.endpoint("collection-item.js:createEndpoint")
  EndpointCreate(...EndpointCreateRequest): EndpointCreateResponse;
  @DataClient.endpoint("collection-item.js:createEndpoint")
  EndpointDuplicate(...EndpointDuplicateRequest): EndpointDuplicateResponse;
}
