using DevTools;

namespace API.Collection.Item.Response;

@DataClient.entity
@parent(Example.Example)
model Response {
  @key responseId: Resource.Id;
  status: int32;
  body: bytes;
  time: Protobuf.WellKnown.Timestamp;
  duration: int32;
  size: int32;
  headerCount: int32;
  assertCount: int32;
}

model ResponseGetRequest is T.Resource.Get.Request<Response>;
model ResponseGetResponse is T.Resource.Get.Response<Response>;

@DataClient.entity
@parent(Response)
model ResponseHeader {
  @key responseHeaderId: Resource.Id;
  key: string;
  value: string;
}

model ResponseHeaderListRequest is T.Resource.List.Request<ResponseHeader>;
model ResponseHeaderListResponse
  is T.Resource.List.Response<TRequest = ResponseHeaderListRequest, TItem = ResponseHeaderListItem>;
model ResponseHeaderListItem is T.Resource.List.Item<ResponseHeader>;

@parent(Response)
model ResponseAssert {
  assert: Request.Assert;
  result: boolean;
}

model ResponseAssertListRequest is T.Resource.List.Request<ResponseAssert>;
model ResponseAssertListResponse
  is T.Resource.List.Response<TRequest = ResponseAssertListRequest, TItem = ResponseAssertListItem>;
model ResponseAssertListItem is T.Resource.List.Item<ResponseAssert>;

interface ResponseService
  extends T.Resource.Get.Interface<
      TResource = Response,
      TRequest = ResponseGetRequest,
      TResponse = ResponseGetResponse
    >,
    T.Resource.List.Interface<
      TResource = ResponseHeader,
      TRequest = ResponseHeaderListRequest,
      TResponse = ResponseHeaderListResponse,
      TItem = ResponseHeaderListItem
    >,
    T.Resource.List.Interface<
      TResource = ResponseAssert,
      TRequest = ResponseAssertListRequest,
      TResponse = ResponseAssertListResponse,
      TItem = ResponseAssertListItem
    > {}
