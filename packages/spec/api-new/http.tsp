using DevTools;

namespace Api.Http;

enum HttpMethod {
  Get,
  Post,
  Put,
  Patch,
  Delete,
  Head,
  Option,
  Connect,
}

// https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Methods/POST
enum HttpBodyKind {
  FormData, // multipart/form-data
  UrlEncoded, // application/x-www-form-urlencoded
  Raw,
}

@withDelta
@TanStackDB.collection
model Http {
  @primaryKey httpId: Id;
  name: string;
  method: HttpMethod;
  url: string;
  bodyKind: HttpBodyKind;
}

model HttpDuplicateRequest {
  httpId: Id;
}

op HttpDuplicate(...HttpDuplicateRequest): {};

model HttpRunRequest {
  httpId: Id;
}

op HttpRun(...HttpRunRequest): {};

@TanStackDB.collection(#{ isReadOnly: true })
model HttpVersion {
  @primaryKey httpVersionId: Id;
  @foreignKey httpId: Id;
}

@withDelta
@TanStackDB.collection
model HttpSearchParam {
  @primaryKey httpSearchParamId: Id;
  ...CommonTableFields<Http>;
}

@withDelta
@TanStackDB.collection
model HttpHeader {
  @primaryKey httpHeaderId: Id;
  ...CommonTableFields<Http>;
}

@withDelta
@TanStackDB.collection
model HttpBodyFormData {
  @primaryKey httpBodyFormDataId: Id;
  ...CommonTableFields<Http>;
}

@withDelta
@TanStackDB.collection
model HttpBodyUrlEncoded {
  @primaryKey httpBodyUrlEncodedId: Id;
  ...CommonTableFields<Http>;
}

@TanStackDB.collection(#{ canDelete: false })
model HttpBodyRaw {
  @primaryKey httpId: Id;
  data: string;
}

@withDelta
@TanStackDB.collection
model HttpAssert {
  @primaryKey httpAssertId: Id;
  @foreignKey httpId: Id;
  value: string;
}

@TanStackDB.collection(#{ isReadOnly: true })
model HttpResponse {
  @primaryKey httpResponseId: Id;
  @foreignKey httpId: Id;
  status: int32;
  body: string;
  time: Protobuf.WellKnown.Timestamp;
  duration: int32;
  size: int32;
}

@TanStackDB.collection(#{ isReadOnly: true })
model HttpResponseHeader {
  @primaryKey httpResponseHeaderId: Id;
  @foreignKey httpId: Id;
  key: string;
  value: string;
}

@TanStackDB.collection(#{ isReadOnly: true })
model HttpResponseAssert {
  @primaryKey httpResponseAssertId: Id;
  @foreignKey httpId: Id;
  value: string;
  success: boolean;
}
