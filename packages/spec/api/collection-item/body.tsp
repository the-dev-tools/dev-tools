using DevTools;

@DevTools.project
namespace API.Collection.Item.Body;

enum BodyKind {
  FormArray,
  UrlEncodedArray,
  Raw,
}

@DataClient.entity
@parent(Example.Example)
model BodyForm {
  @key bodyId: Resource.Id;
  key: string;
  enabled: boolean;
  value: string;
  description: string;
}

model BodyFormListRequest is T.Resource.List.Request<BodyForm>;
model BodyFormListResponse is T.Resource.List.Response<TRequest = BodyFormListRequest, TItem = BodyFormListItem>;
model BodyFormListItem is T.Resource.List.Item<BodyForm>;

model BodyFormCreateRequest is T.Resource.Create.Request<BodyForm>;
model BodyFormCreateResponse is T.Resource.Create.Response<BodyForm>;

model BodyFormUpdateRequest is T.Resource.Update.Request<BodyForm>;
model BodyFormUpdateResponse is T.Resource.Update.Response<BodyForm>;

model BodyFormDeleteRequest is T.Resource.Delete.Request<BodyForm>;
model BodyFormDeleteResponse is T.Resource.Delete.Response<BodyForm>;

model BodyFormMoveRequest {
  ...ParentKeyOf<BodyForm>;
  ...KeyOf<BodyForm>;
  position: Resource.MovePosition;
  targetBodyId: Resource.Id;
}

model BodyFormDelta is T.Delta.Item<BodyForm>;

model BodyFormDeltaListRequest is T.Delta.List.Request<BodyFormDelta>;
model BodyFormDeltaListResponse
  is T.Delta.List.Response<TRequest = BodyFormDeltaListRequest, TItem = BodyFormDeltaListItem>;
model BodyFormDeltaListItem is T.Delta.List.Item<BodyFormDelta>;

model BodyFormDeltaCreateRequest is T.Delta.Create.Request<BodyFormDelta>;
model BodyFormDeltaCreateResponse is T.Delta.Create.Response<BodyFormDelta>;

model BodyFormDeltaUpdateRequest is T.Delta.Update.Request<BodyFormDelta>;
model BodyFormDeltaUpdateResponse is T.Delta.Update.Response<BodyFormDelta>;

model BodyFormDeltaDeleteRequest is T.Delta.Delete.Request<BodyFormDelta>;
model BodyFormDeltaDeleteResponse is T.Delta.Delete.Response<BodyFormDelta>;

model BodyFormDeltaResetRequest is T.Delta.Reset.Request<BodyFormDelta>;
model BodyFormDeltaResetResponse is T.Delta.Reset.Response<BodyFormDelta>;

model BodyFormDeltaMoveRequest {
  ...ParentKeyOf<BodyFormDelta>;
  ...KeyOf<BodyFormDelta>;
  originId: Resource.Id;
  position: Resource.MovePosition;
  targetBodyId: Resource.Id;
}

@DataClient.entity
@parent(Example.Example)
model BodyUrlEncoded {
  @key bodyId: Resource.Id;
  key: string;
  enabled: boolean;
  value: string;
  description: string;
}

model BodyUrlEncodedListRequest is T.Resource.List.Request<BodyUrlEncoded>;
model BodyUrlEncodedListResponse
  is T.Resource.List.Response<TRequest = BodyUrlEncodedListRequest, TItem = BodyUrlEncodedListItem>;
model BodyUrlEncodedListItem is T.Resource.List.Item<BodyUrlEncoded>;

model BodyUrlEncodedCreateRequest is T.Resource.Create.Request<BodyUrlEncoded>;
model BodyUrlEncodedCreateResponse is T.Resource.Create.Response<BodyUrlEncoded>;

model BodyUrlEncodedUpdateRequest is T.Resource.Update.Request<BodyUrlEncoded>;
model BodyUrlEncodedUpdateResponse is T.Resource.Update.Response<BodyUrlEncoded>;

model BodyUrlEncodedDeleteRequest is T.Resource.Delete.Request<BodyUrlEncoded>;
model BodyUrlEncodedDeleteResponse is T.Resource.Delete.Response<BodyUrlEncoded>;

model BodyUrlEncodedMoveRequest {
  ...ParentKeyOf<BodyUrlEncoded>;
  ...KeyOf<BodyUrlEncoded>;
  position: Resource.MovePosition;
  targetBodyId: Resource.Id;
}

model BodyUrlEncodedDelta is T.Delta.Item<BodyUrlEncoded>;

model BodyUrlEncodedDeltaListRequest is T.Delta.List.Request<BodyUrlEncodedDelta>;
model BodyUrlEncodedDeltaListResponse
  is T.Delta.List.Response<TRequest = BodyUrlEncodedDeltaListRequest, TItem = BodyUrlEncodedDeltaListItem>;
model BodyUrlEncodedDeltaListItem is T.Delta.List.Item<BodyUrlEncodedDelta>;

model BodyUrlEncodedDeltaCreateRequest is T.Delta.Create.Request<BodyUrlEncodedDelta>;
model BodyUrlEncodedDeltaCreateResponse is T.Delta.Create.Response<BodyUrlEncodedDelta>;

model BodyUrlEncodedDeltaUpdateRequest is T.Delta.Update.Request<BodyUrlEncodedDelta>;
model BodyUrlEncodedDeltaUpdateResponse is T.Delta.Update.Response<BodyUrlEncodedDelta>;

model BodyUrlEncodedDeltaDeleteRequest is T.Delta.Delete.Request<BodyUrlEncodedDelta>;
model BodyUrlEncodedDeltaDeleteResponse is T.Delta.Delete.Response<BodyUrlEncodedDelta>;

model BodyUrlEncodedDeltaResetRequest is T.Delta.Reset.Request<BodyUrlEncodedDelta>;
model BodyUrlEncodedDeltaResetResponse is T.Delta.Reset.Response<BodyUrlEncodedDelta>;

model BodyUrlEncodedDeltaMoveRequest {
  ...ParentKeyOf<BodyUrlEncodedDelta>;
  ...KeyOf<BodyUrlEncodedDelta>;
  originId: Resource.Id;
  position: Resource.MovePosition;
  targetBodyId: Resource.Id;
}

@DataClient.entity
model BodyRaw {
  ...KeyOf<Example.Example>;
  data: bytes;
}

model BodyRawGetRequest is T.Resource.Get.Request<BodyRaw>;
model BodyRawGetResponse is T.Resource.Get.Response<BodyRaw>;

model BodyRawUpdateRequest is T.Resource.Update.Request<BodyRaw>;
model BodyRawUpdateResponse is T.Resource.Update.Response<BodyRaw>;

interface BodyService
  extends BlankInterface,
    // Form Array
    T.Resource.List.Interface<
      TResource = BodyForm,
      TRequest = BodyFormListRequest,
      TResponse = BodyFormListResponse,
      TItem = BodyFormListItem
    >,
    T.Resource.Create.Interface<
      TResource = BodyForm,
      TRequest = BodyFormCreateRequest,
      TResponse = BodyFormCreateResponse,
      TListRequest = BodyFormListRequest,
      TListItem = BodyFormListItem
    >,
    T.Resource.Update.Interface<
      TResource = BodyForm,
      TRequest = BodyFormUpdateRequest,
      TResponse = BodyFormUpdateResponse
    >,
    T.Resource.Delete.Interface<
      TResource = BodyForm,
      TRequest = BodyFormDeleteRequest,
      TResponse = BodyFormDeleteResponse
    >,
    T.Delta.Interface<
      TResource = BodyFormDelta,
      // List
      TListRequest = BodyFormDeltaListRequest,
      TListResponse = BodyFormDeltaListResponse,
      TListItem = BodyFormDeltaListItem,
      // Create
      TCreateRequest = BodyFormDeltaCreateRequest,
      TCreateResponse = BodyFormDeltaCreateResponse,
      // Update
      TUpdateRequest = BodyFormDeltaUpdateRequest,
      TUpdateResponse = BodyFormDeltaUpdateResponse,
      // Delete
      TDeleteRequest = BodyFormDeltaDeleteRequest,
      TDeleteResponse = BodyFormDeltaDeleteResponse,
      // Reset
      TResetRequest = BodyFormDeltaResetRequest,
      TResetResponse = BodyFormDeltaResetResponse
    >,
    // URL Encoded Array
    T.Resource.List.Interface<
      TResource = BodyUrlEncoded,
      TRequest = BodyUrlEncodedListRequest,
      TResponse = BodyUrlEncodedListResponse,
      TItem = BodyUrlEncodedListItem
    >,
    T.Resource.Create.Interface<
      TResource = BodyUrlEncoded,
      TRequest = BodyUrlEncodedCreateRequest,
      TResponse = BodyUrlEncodedCreateResponse,
      TListRequest = BodyUrlEncodedListRequest,
      TListItem = BodyUrlEncodedListItem
    >,
    T.Resource.Update.Interface<
      TResource = BodyUrlEncoded,
      TRequest = BodyUrlEncodedUpdateRequest,
      TResponse = BodyUrlEncodedUpdateResponse
    >,
    T.Resource.Delete.Interface<
      TResource = BodyUrlEncoded,
      TRequest = BodyUrlEncodedDeleteRequest,
      TResponse = BodyUrlEncodedDeleteResponse
    >,
    T.Delta.Interface<
      TResource = BodyUrlEncodedDelta,
      // List
      TListRequest = BodyUrlEncodedDeltaListRequest,
      TListResponse = BodyUrlEncodedDeltaListResponse,
      TListItem = BodyUrlEncodedDeltaListItem,
      // Create
      TCreateRequest = BodyUrlEncodedDeltaCreateRequest,
      TCreateResponse = BodyUrlEncodedDeltaCreateResponse,
      // Update
      TUpdateRequest = BodyUrlEncodedDeltaUpdateRequest,
      TUpdateResponse = BodyUrlEncodedDeltaUpdateResponse,
      // Delete
      TDeleteRequest = BodyUrlEncodedDeltaDeleteRequest,
      TDeleteResponse = BodyUrlEncodedDeltaDeleteResponse,
      // Reset
      TResetRequest = BodyUrlEncodedDeltaResetRequest,
      TResetResponse = BodyUrlEncodedDeltaResetResponse
    >,
    // Raw
    T.Resource.Get.Interface<TResource = BodyRaw, TRequest = BodyRawGetRequest, TResponse = BodyRawGetResponse>,
    T.Resource.Update.Interface<
      TResource = BodyRaw,
      TRequest = BodyRawUpdateRequest,
      TResponse = BodyRawUpdateResponse
    > {
  @DataClient.endpoint("request.js:moveBodyForm")
  BodyFormMove(...BodyFormMoveRequest): {};
  @DataClient.endpoint("request.js:moveBodyFormDelta")
  BodyFormDeltaMove(...BodyFormDeltaMoveRequest): {};
  @DataClient.endpoint("request.js:moveBodyUrlEncoded")
  BodyUrlEncodedMove(...BodyUrlEncodedMoveRequest): {};
  @DataClient.endpoint("request.js:moveBodyUrlEncodedDelta")
  BodyUrlEncodedDeltaMove(...BodyUrlEncodedDeltaMoveRequest): {};
}
