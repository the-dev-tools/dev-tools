import "@typespec/protobuf";
import "@typespec/rest";

import "../../lib";

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
@parentResource(Example.Example)
model BodyFormItem {
  @key bodyId: Resource.Id;
  @visibility("query") parentBodyId?: Resource.Id;
  key: string;
  enabled: boolean;
  value: string;
  description: string;
}

@autoFields
@normalize
@parentResource(Example.Example)
model BodyUrlEncodedItem {
  @key bodyId: Resource.Id;
  @visibility("query") parentBodyId?: Resource.Id;
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

// Form Array
model BodyFormItemChange is Resource.Change<BodyFormItem>;
model BodyFormItemListItem is Resource.List.Item<BodyFormItem>;
model BodyFormItemListRequest is Resource.List.Request<BodyFormItem>;
model BodyFormItemListResponse is Resource.List.Response<BodyFormItem, BodyFormItemListItem>;
model BodyFormItemCreateRequest is Resource.Create.Request<BodyFormItem>;
model BodyFormItemCreateResponse is Resource.Create.Response<BodyFormItem, BodyFormItemListResponse>;
model BodyFormItemUpdateRequest is Resource.Update.Request<BodyFormItem>;
model BodyFormItemUpdateResponse is Resource.Update.Response<BodyFormItem>;
model BodyFormItemDeleteRequest is Resource.Delete.Request<BodyFormItem>;
model BodyFormItemDeleteResponse is Resource.Delete.Response<BodyFormItem>;

// URL Encoded Array
model BodyUrlEncodedItemChange is Resource.Change<BodyUrlEncodedItem>;
model BodyUrlEncodedItemListItem is Resource.List.Item<BodyUrlEncodedItem>;
model BodyUrlEncodedItemListRequest is Resource.List.Request<BodyUrlEncodedItem>;
model BodyUrlEncodedItemListResponse is Resource.List.Response<BodyUrlEncodedItem, BodyUrlEncodedItemListItem>;
model BodyUrlEncodedItemCreateRequest is Resource.Create.Request<BodyUrlEncodedItem>;
model BodyUrlEncodedItemCreateResponse is Resource.Create.Response<BodyUrlEncodedItem, BodyUrlEncodedItemListResponse>;
model BodyUrlEncodedItemUpdateRequest is Resource.Update.Request<BodyUrlEncodedItem>;
model BodyUrlEncodedItemUpdateResponse is Resource.Update.Response<BodyUrlEncodedItem>;
model BodyUrlEncodedItemDeleteRequest is Resource.Delete.Request<BodyUrlEncodedItem>;
model BodyUrlEncodedItemDeleteResponse is Resource.Delete.Response<BodyUrlEncodedItem>;

// Raw
model BodyRawGetRequest is Resource.Get.Request<BodyRaw> {
  ...ParentKeyOf<BodyRaw>;
}
model BodyRawGetResponse is Resource.Get.Response<BodyRaw>;
model BodyRawUpdateRequest is Resource.Update.Request<BodyRaw>;
model BodyRawUpdateResponse is Resource.Update.Response<BodyRaw>;

@Protobuf.service
interface BodyService {
  // Form Array
  BodyFormItemList(...BodyFormItemListRequest): BodyFormItemListResponse;
  BodyFormItemCreate(...BodyFormItemCreateRequest): BodyFormItemCreateResponse;
  BodyFormItemUpdate(...BodyFormItemUpdateRequest): BodyFormItemUpdateResponse;
  BodyFormItemDelete(...BodyFormItemDeleteRequest): BodyFormItemDeleteResponse;

  // URL Encoded Array
  BodyUrlEncodedItemList(...BodyUrlEncodedItemListRequest): BodyUrlEncodedItemListResponse;
  BodyUrlEncodedItemCreate(...BodyUrlEncodedItemCreateRequest): BodyUrlEncodedItemCreateResponse;
  BodyUrlEncodedItemUpdate(...BodyUrlEncodedItemUpdateRequest): BodyUrlEncodedItemUpdateResponse;
  BodyUrlEncodedItemDelete(...BodyUrlEncodedItemDeleteRequest): BodyUrlEncodedItemDeleteResponse;

  // Raw
  BodyRawGet(...BodyRawGetRequest): BodyRawGetResponse;
  BodyRawUpdate(...BodyRawUpdateRequest): BodyRawUpdateResponse;
}
