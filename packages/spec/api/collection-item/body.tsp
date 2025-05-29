import "@typespec/protobuf";
import "@typespec/rest";

import "../lib.tsp";

import "../delta.tsp";
import "./example.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "collection.item.body.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/collection/item/body/v1;bodyv1",
  },
})
namespace API.CollectionItem.Body;

enum BodyKind {
  BODY_KIND_UNSPECIFIED: 0,
  BODY_KIND_FORM_ARRAY: 1,
  BODY_KIND_URL_ENCODED_ARRAY: 2,
  BODY_KIND_RAW: 3,
}

@autoFields
@normalize
@entity
@parentResource(Example.Example)
model BodyForm {
  @key bodyId: Resource.Id;
  key: string;
  enabled: boolean;
  value: string;
  description: string;
}

@autoFields
@normalize
@entity
@parentResource(Example.Example)
model BodyUrlEncoded {
  @key bodyId: Resource.Id;
  key: string;
  enabled: boolean;
  value: string;
  description: string;
}

@autoFields
@parentResource(Example.Example)
model BodyRaw {
  data: bytes;
}

// Raw
model BodyRawGetRequest is Resource.Get.Request<BodyRaw> {
  ...ParentKeyOf<BodyRaw>;
}
model BodyRawGetResponse is Resource.Get.Response<BodyRaw>;
model BodyRawUpdateRequest is Resource.Update.Request<BodyRaw> {
  ...ParentKeyOf<BodyRaw>;
}
model BodyRawUpdateResponse is Resource.Update.Response<BodyRaw>;

@Protobuf.service
interface BodyService
  extends BlankInterface,
    // Form Array
    Resource.List.Interface<BodyForm>,
    Resource.Create.Interface<BodyForm>,
    Resource.Update.Interface<BodyForm>,
    Resource.Delete.Interface<BodyForm>,
    Delta.Interface<BodyForm>,
    // URL Encoded Array
    Resource.List.Interface<BodyUrlEncoded>,
    Resource.Create.Interface<BodyUrlEncoded>,
    Resource.Update.Interface<BodyUrlEncoded>,
    Resource.Delete.Interface<BodyUrlEncoded>,
    Delta.Interface<BodyUrlEncoded> {
  // Raw
  BodyRawGet(...BodyRawGetRequest): BodyRawGetResponse;
  BodyRawUpdate(...BodyRawUpdateRequest): BodyRawUpdateResponse;
}
