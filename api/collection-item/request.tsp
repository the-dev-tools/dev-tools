import "@typespec/protobuf";
import "@typespec/rest";

import "./example.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "collection.item.request.v1",
  options: {
    go_package: "dev-tools-services/gen/collection/item/request/v1;requestv1",
  },
})
namespace API.CollectionItem.Request;

@parentResource(Example.Example)
model Query {
  @field(5) @key queryId: string;
  @field(6) key: string;
  @field(7) enabled: boolean;
  @field(8) value: string;
  @field(9) description: string;
}

@parentResource(Example.Example)
model Header {
  @field(5) @key headerId: string;
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
  @field(5) @key bodyId: string;
  @field(6) key: string;
  @field(7) enabled: boolean;
  @field(8) value: string;
  @field(9) description: string;
}

@parentResource(Example.Example)
model BodyUrlEncodedItem {
  @field(5) @key bodyId: string;
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
  @field(5) @key assertId: string;
  @field(6) name: string;
  @field(7) value: string;
  @field(8) type: AssertType;
  @field(9) target: AssertTarget;
}

// Query
model ListQueriesItem is Resource.List.Item<Query>;
model ListQueriesRequest is Resource.List.Request<ListQueriesItem>;
model ListQueriesResponse is Resource.List.Response<Query>;
model CreateQueryRequest is Resource.Create.Request<Query>;
model CreateQueryResponse is Resource.Create.Response<Query>;
model UpdateQueryRequest is Resource.Update.Request<Query>;
model UpdateQueryResponse is Resource.Update.Response<Query>;
model DeleteQueryRequest is Resource.Delete.Request<Query>;
model DeleteQueryResponse is Resource.Delete.Response<Query>;

// Header
model ListHeadersItem is Resource.List.Item<Header>;
model ListHeadersRequest is Resource.List.Request<ListHeadersItem>;
model ListHeadersResponse is Resource.List.Response<Header>;
model CreateHeaderRequest is Resource.Create.Request<Header>;
model CreateHeaderResponse is Resource.Create.Response<Header>;
model UpdateHeaderRequest is Resource.Update.Request<Header>;
model UpdateHeaderResponse is Resource.Update.Response<Header>;
model DeleteHeaderRequest is Resource.Delete.Request<Header>;
model DeleteHeaderResponse is Resource.Delete.Response<Header>;

// Assert
model ListAssertsItem is Resource.List.Item<Assert>;
model ListAssertsRequest is Resource.List.Request<ListAssertsItem>;
model ListAssertsResponse is Resource.List.Response<Assert>;
model CreateAssertRequest is Resource.Create.Request<Assert>;
model CreateAssertResponse is Resource.Create.Response<Assert>;
model UpdateAssertRequest is Resource.Update.Request<Assert>;
model UpdateAssertResponse is Resource.Update.Response<Assert>;
model DeleteAssertRequest is Resource.Delete.Request<Assert>;
model DeleteAssertResponse is Resource.Delete.Response<Assert>;

// Body - Form Array
model ListBodyFormItemsItem is Resource.List.Item<BodyFormItem>;
model ListBodyFormItemsRequest is Resource.List.Request<ListBodyFormItemsItem>;
model ListBodyFormItemsResponse is Resource.List.Response<BodyFormItem>;
model CreateBodyFormItemRequest is Resource.Create.Request<BodyFormItem>;
model CreateBodyFormItemResponse is Resource.Create.Response<BodyFormItem>;
model UpdateBodyFormItemRequest is Resource.Update.Request<BodyFormItem>;
model UpdateBodyFormItemResponse is Resource.Update.Response<BodyFormItem>;
model DeleteBodyFormItemRequest is Resource.Delete.Request<BodyFormItem>;
model DeleteBodyFormItemResponse is Resource.Delete.Response<BodyFormItem>;

// Body - URL Encoded Array
model ListBodyUrlEncodedItemsItem is Resource.List.Item<BodyUrlEncodedItem>;
model ListBodyUrlEncodedItemsRequest is Resource.List.Request<ListBodyUrlEncodedItemsItem>;
model ListBodyUrlEncodedItemsResponse is Resource.List.Response<BodyUrlEncodedItem>;
model CreateBodyUrlEncodedItemRequest is Resource.Create.Request<BodyUrlEncodedItem>;
model CreateBodyUrlEncodedItemResponse is Resource.Create.Response<BodyUrlEncodedItem>;
model UpdateBodyUrlEncodedItemRequest is Resource.Update.Request<BodyUrlEncodedItem>;
model UpdateBodyUrlEncodedItemResponse is Resource.Update.Response<BodyUrlEncodedItem>;
model DeleteBodyUrlEncodedItemRequest is Resource.Delete.Request<BodyUrlEncodedItem>;
model DeleteBodyUrlEncodedItemResponse is Resource.Delete.Response<BodyUrlEncodedItem>;

// Body - Raw
model GetBodyRawRequest is Resource.Get.Request<BodyRaw> {
  ...ParentKeyOf<BodyRaw>;
}
model GetBodyRawResponse is Resource.Get.Response<BodyRaw>;
model UpdateBodyRawRequest is Resource.Update.Request<BodyRaw> {
  ...ParentKeyOf<BodyRaw>;
}
model UpdateBodyRawResponse is Resource.Update.Response<BodyRaw>;

@Protobuf.service
interface CollectionItemRequestService {
  // Query
  ListQueries(...ListQueriesRequest): ListQueriesResponse;
  CreateQuery(...CreateQueryRequest): CreateQueryResponse;
  UpdateQuery(...UpdateQueryRequest): UpdateQueryResponse;
  DeleteQuery(...DeleteQueryRequest): DeleteQueryResponse;

  // Header
  ListHeaders(...ListHeadersRequest): ListHeadersResponse;
  CreateHeader(...CreateHeaderRequest): CreateHeaderResponse;
  UpdateHeader(...UpdateHeaderRequest): UpdateHeaderResponse;
  DeleteHeader(...DeleteHeaderRequest): DeleteHeaderResponse;

  // Assert
  ListAsserts(...ListAssertsRequest): ListAssertsResponse;
  CreateAssert(...CreateAssertRequest): CreateAssertResponse;
  UpdateAssert(...UpdateAssertRequest): UpdateAssertResponse;
  DeleteAssert(...DeleteAssertRequest): DeleteAssertResponse;

  // Body - Form Array
  ListBodyFormItems(...ListBodyFormItemsRequest): ListBodyFormItemsResponse;
  CreateBodyFormItem(...CreateBodyFormItemRequest): CreateBodyFormItemResponse;
  UpdateBodyFormItem(...UpdateBodyFormItemRequest): UpdateBodyFormItemResponse;
  DeleteBodyFormItem(...DeleteBodyFormItemRequest): DeleteBodyFormItemResponse;

  // Body - URL Encoded Array
  ListBodyUrlEncodedItems(...ListBodyUrlEncodedItemsRequest): ListBodyUrlEncodedItemsResponse;
  CreateBodyUrlEncodedItem(...CreateBodyUrlEncodedItemRequest): CreateBodyUrlEncodedItemResponse;
  UpdateBodyUrlEncodedItem(...UpdateBodyUrlEncodedItemRequest): UpdateBodyUrlEncodedItemResponse;
  DeleteBodyUrlEncodedItem(...DeleteBodyUrlEncodedItemRequest): DeleteBodyUrlEncodedItemResponse;

  // Body - Raw
  GetBodyRaw(...GetBodyRawRequest): GetBodyRawResponse;
  UpdateBodyRaw(...UpdateBodyRawRequest): UpdateBodyRawResponse;
}
