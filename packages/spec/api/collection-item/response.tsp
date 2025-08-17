using DevTools;

namespace API.CollectionItem.Response;

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

@DataClient.entity
@parent(Response)
model ResponseHeader {
  @key responseHeaderId: Resource.Id;
  key: string;
  value: string;
}

@parent(Response)
model ResponseAssert {
  @key responseAssertId: Resource.Id;
  assert: Request.Assert;
  result: boolean;
}

interface ResponseService
  extends Resource.Get.Interface<Response>,
    Resource.List.Interface<ResponseHeader>,
    Resource.List.Interface<ResponseAssert> {}
