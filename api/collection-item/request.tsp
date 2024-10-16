import "@typespec/protobuf";
import "@typespec/rest";

import "./example.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "collection.item.request.v1",
  options: {
    go_package: "dev-tools-spec/dist/buf/go/collection/item/request/v1;requestv1",
  },
})
namespace API.CollectionItem.Request;

@parentResource(Example.Example)
model Query {
  @field(5) @key queryId: Resource.ID;
  @field(6) key: string;
  @field(7) enabled: boolean;
  @field(8) value: string;
  @field(9) description: string;
}

@parentResource(Example.Example)
model Header {
  @field(5) @key headerId: Resource.ID;
  @field(6) key: string;
  @field(7) enabled: boolean;
  @field(8) value: string;
  @field(9) description: string;
}

enum BodyKind {
  BODY_KIND_UNSPECIFIED: 0,
  BODY_KIND_FORM_ARRAY: 1,
  BODY_KIND_URL_ENCODED_ARRAY: 2,
  BODY_KIND_RAW: 3,
}

@parentResource(Example.Example)
model BodyFormItem {
  @field(5) @key bodyId: Resource.ID;
  @field(6) key: string;
  @field(7) enabled: boolean;
  @field(8) value: string;
  @field(9) description: string;
}

@parentResource(Example.Example)
model BodyUrlEncodedItem {
  @field(5) @key bodyId: Resource.ID;
  @field(6) key: string;
  @field(7) enabled: boolean;
  @field(8) value: string;
  @field(9) description: string;
}

@parentResource(Example.Example)
model BodyRaw {
  @field(6) data: bytes;
}

enum AssertType {
  ASSERT_TYPE_UNSPECIFIED: 0,
  ASSERT_TYPE_EQUAL: 1,
  ASSERT_TYPE_NOT_EQUAL: 2,
  ASSERT_TYPE_CONTAINS: 3,
  ASSERT_TYPE_NOT_CONTAINS: 4,
  ASSERT_TYPE_GREATER: 5,
  ASSERT_TYPE_ASSERT_TYPE_LESS: 6,
  ASSERT_TYPE_GREATER_OR_EQUAL: 7,
  ASSERT_TYPE_LESS_OR_EQUAL: 8,
}

enum AssertTarget {
  ASSERT_TARGET_UNSPECIFIED: 0,
  ASSERT_TARGET_HEADER: 1,
  ASSERT_TARGET_BODY: 2,
}

@parentResource(Example.Example)
model Assert {
  @field(5) @key assertId: Resource.ID;
  @field(6) name: string;
  @field(7) value: string;
  @field(8) type: AssertType;
  @field(9) target: AssertTarget;
}

// Query
model QueryListItem is Resource.List.Item<Query>;
model QueryListRequest is Resource.List.Request<Query>;
model QueryListResponse is Resource.List.Response<QueryListItem>;
model QueryCreateRequest is Resource.Create.Request<Query>;
model QueryCreateResponse is Resource.Create.Response<Query>;
model QueryUpdateRequest is Resource.Update.Request<Query>;
model QueryUpdateResponse is Resource.Update.Response<Query>;
model QueryDeleteRequest is Resource.Delete.Request<Query>;
model QueryDeleteResponse is Resource.Delete.Response<Query>;

// Header
model HeaderListItem is Resource.List.Item<Header>;
model HeaderListRequest is Resource.List.Request<Header>;
model HeaderListResponse is Resource.List.Response<HeaderListItem>;
model HeaderCreateRequest is Resource.Create.Request<Header>;
model HeaderCreateResponse is Resource.Create.Response<Header>;
model HeaderUpdateRequest is Resource.Update.Request<Header>;
model HeaderUpdateResponse is Resource.Update.Response<Header>;
model HeaderDeleteRequest is Resource.Delete.Request<Header>;
model HeaderDeleteResponse is Resource.Delete.Response<Header>;

// Assert
model AssertListItem is Resource.List.Item<Assert>;
model AssertListRequest is Resource.List.Request<Assert>;
model AssertListResponse is Resource.List.Response<AssertListItem>;
model AssertCreateRequest is Resource.Create.Request<Assert>;
model AssertCreateResponse is Resource.Create.Response<Assert>;
model AssertUpdateRequest is Resource.Update.Request<Assert>;
model AssertUpdateResponse is Resource.Update.Response<Assert>;
model AssertDeleteRequest is Resource.Delete.Request<Assert>;
model AssertDeleteResponse is Resource.Delete.Response<Assert>;

// Body - Form Array
model BodyFormItemListItem is Resource.List.Item<BodyFormItem>;
model BodyFormItemListRequest is Resource.List.Request<BodyFormItem>;
model BodyFormItemListResponse is Resource.List.Response<BodyFormItemListItem>;
model BodyFormItemCreateRequest is Resource.Create.Request<BodyFormItem>;
model BodyFormItemCreateResponse is Resource.Create.Response<BodyFormItem>;
model BodyFormItemUpdateRequest is Resource.Update.Request<BodyFormItem>;
model BodyFormItemUpdateResponse is Resource.Update.Response<BodyFormItem>;
model BodyFormItemDeleteRequest is Resource.Delete.Request<BodyFormItem>;
model BodyFormItemDeleteResponse is Resource.Delete.Response<BodyFormItem>;

// Body - URL Encoded Array
model BodyUrlEncodedItemListItem is Resource.List.Item<BodyUrlEncodedItem>;
model BodyUrlEncodedItemListRequest is Resource.List.Request<BodyUrlEncodedItem>;
model BodyUrlEncodedItemListResponse is Resource.List.Response<BodyUrlEncodedItemListItem>;
model BodyUrlEncodedItemCreateRequest is Resource.Create.Request<BodyUrlEncodedItem>;
model BodyUrlEncodedItemCreateResponse is Resource.Create.Response<BodyUrlEncodedItem>;
model BodyUrlEncodedItemUpdateRequest is Resource.Update.Request<BodyUrlEncodedItem>;
model BodyUrlEncodedItemUpdateResponse is Resource.Update.Response<BodyUrlEncodedItem>;
model BodyUrlEncodedItemDeleteRequest is Resource.Delete.Request<BodyUrlEncodedItem>;
model BodyUrlEncodedItemDeleteResponse is Resource.Delete.Response<BodyUrlEncodedItem>;

// Body - Raw
model BodyRawGetRequest is Resource.Get.Request<BodyRaw> {
  ...ParentKeyOf<BodyRaw>;
}
model BodyRawGetResponse is Resource.Get.Response<BodyRaw>;
model BodyRawUpdateRequest is Resource.Update.Request<BodyRaw> {
  ...ParentKeyOf<BodyRaw>;
}
model BodyRawUpdateResponse is Resource.Update.Response<BodyRaw>;

@Protobuf.service
interface RequestService {
  // Query
  QueryList(...QueryListRequest): QueryListResponse;
  QueryCreate(...QueryCreateRequest): QueryCreateResponse;
  QueryUpdate(...QueryUpdateRequest): QueryUpdateResponse;
  QueryDelete(...QueryDeleteRequest): QueryDeleteResponse;

  // Header
  HeaderList(...HeaderListRequest): HeaderListResponse;
  HeaderCreate(...HeaderCreateRequest): HeaderCreateResponse;
  HeaderUpdate(...HeaderUpdateRequest): HeaderUpdateResponse;
  HeaderDelete(...HeaderDeleteRequest): HeaderDeleteResponse;

  // Assert
  AssertList(...AssertListRequest): AssertListResponse;
  AssertCreate(...AssertCreateRequest): AssertCreateResponse;
  AssertUpdate(...AssertUpdateRequest): AssertUpdateResponse;
  AssertDelete(...AssertDeleteRequest): AssertDeleteResponse;

  // Body - Form Array
  BodyFormItemList(...BodyFormItemListRequest): BodyFormItemListResponse;
  BodyFormItemCreate(...BodyFormItemCreateRequest): BodyFormItemCreateResponse;
  BodyFormItemUpdate(...BodyFormItemUpdateRequest): BodyFormItemUpdateResponse;
  BodyFormItemDelete(...BodyFormItemDeleteRequest): BodyFormItemDeleteResponse;

  // Body - URL Encoded Array
  BodyUrlEncodedItemList(...BodyUrlEncodedItemListRequest): BodyUrlEncodedItemListResponse;
  BodyUrlEncodedItemCreate(...BodyUrlEncodedItemCreateRequest): BodyUrlEncodedItemCreateResponse;
  BodyUrlEncodedItemUpdate(...BodyUrlEncodedItemUpdateRequest): BodyUrlEncodedItemUpdateResponse;
  BodyUrlEncodedItemDelete(...BodyUrlEncodedItemDeleteRequest): BodyUrlEncodedItemDeleteResponse;

  // Body - Raw
  BodyRawGet(...BodyRawGetRequest): BodyRawGetResponse;
  BodyRawUpdate(...BodyRawUpdateRequest): BodyRawUpdateResponse;
}
