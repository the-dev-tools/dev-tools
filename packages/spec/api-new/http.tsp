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

enum HttpBodyKind {
  FormArray,
  UrlEncodedArray,
  Raw,
}

// TODO: delta
@TanStackDB.collection
model Http {
  @key httpId: Id;
  name: string;
  method: HttpMethod;
  url: string;
  bodyKind: HttpBodyKind;
}

model HttpRunRequest {
  httpId: Id;
}

op HttpRun(...HttpRunRequest): {};

// TODO: read only
@TanStackDB.collection
model HttpVersion {
  @key httpVersionId: Id;
  httpId: Id;
}

// TODO: delta
@TanStackDB.collection
model HttpSearchParam {
  @key httpSearchParamId: Id;
  httpId: Id;
  ...CommonTableFields;
}

// TODO: delta
@TanStackDB.collection
model HttpHeader {
  @key httpHeaderId: Id;
  httpId: Id;
  ...CommonTableFields;
}

// TODO: delta
@TanStackDB.collection
model HttpBodyForm {
  @key httpBodyFormId: Id;
  httpId: Id;
  ...CommonTableFields;
}

// TODO: delta
@TanStackDB.collection
model HttpBodyUrlEncoded {
  @key httpBodyUrlEncodedId: Id;
  httpId: Id;
  ...CommonTableFields;
}

// TODO: read & update only
@TanStackDB.collection
model HttpBodyRaw {
  @key httpId: Id;
  data: string;
}

// TODO: delta
@TanStackDB.collection
model HttpAssert {
  @key httpAssertId: Id;
  httpId: Id;
  value: string;
}

// TODO: read-only
@TanStackDB.collection
model HttpResponse {
  @key httpResponseId: Id;
  httpId: Id;
  status: int32;
  body: bytes;
  time: Protobuf.WellKnown.Timestamp;
  duration: int32;
  size: int32;
}

// TODO: read-only
@TanStackDB.collection
model HttpResponseHeader {
  @key httpResponseHeaderId: Id;
  httpId: Id;
  key: string;
  value: string;
}

// TODO: read-only
@TanStackDB.collection
model HttpResponseAssert {
  @key httpResponseAssertId: Id;
  httpId: Id;
  value: string;
  success: boolean;
}
