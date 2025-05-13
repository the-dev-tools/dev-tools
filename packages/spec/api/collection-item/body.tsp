import "@typespec/protobuf";
import "@typespec/rest";

import "../lib.tsp";

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
model BodyFormItem {
  @key bodyId: Resource.Id;
  @visibility(Resource.Mutation.Create) parentBodyId?: Resource.Id;
  key: string;
  enabled: boolean;
  value: string;
  description: string;
}

@autoFields
@normalize
@entity
@parentResource(Example.Example)
model BodyUrlEncodedItem {
  @key bodyId: Resource.Id;
  @visibility(Resource.Mutation.Create) parentBodyId?: Resource.Id;
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
    Resource.List.Interface<BodyFormItem>,
    Resource.Create.Interface<BodyFormItem>,
    Resource.Update.Interface<BodyFormItem>,
    Resource.Delete.Interface<BodyFormItem>,
    // URL Encoded Array
    Resource.List.Interface<BodyUrlEncodedItem>,
    Resource.Create.Interface<BodyUrlEncodedItem>,
    Resource.Update.Interface<BodyUrlEncodedItem>,
    Resource.Delete.Interface<BodyUrlEncodedItem> {
  // Raw
  BodyRawGet(...BodyRawGetRequest): BodyRawGetResponse;
  BodyRawUpdate(...BodyRawUpdateRequest): BodyRawUpdateResponse;
}
