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
  @doc("HTTP method for the request (GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS)") method: HttpMethod;
  @doc("The URL to send the HTTP request to") url: string;
  bodyKind: HttpBodyKind;
  lastRunAt?: Protobuf.WellKnown.Timestamp;
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
  @foreignKey deltaHttpId?: Id;
  name: string;
  description: string;
  createdAt: int64;
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

@withDelta
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
  enabled: boolean;
  order: float32;
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
  @foreignKey httpResponseId: Id;
  key: string;
  value: string;
}

@TanStackDB.collection(#{ isReadOnly: true })
model HttpResponseAssert {
  @primaryKey httpResponseAssertId: Id;
  @foreignKey httpResponseId: Id;
  value: string;
  success: boolean;
}
