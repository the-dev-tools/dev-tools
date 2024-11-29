import "@typespec/protobuf";
import "@typespec/rest";

import "../../lib";

import "./example.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "collection.item.request.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/collection/item/request/v1;requestv1",
  },
})
namespace API.CollectionItem.Request;

@autoFields
@parentResource(Example.Example)
model Query {
  @key queryId: Resource.ID;
  key: string;
  enabled: boolean;
  value: string;
  description: string;
}

@autoFields
@parentResource(Example.Example)
model Header {
  @key headerId: Resource.ID;
  key: string;
  enabled: boolean;
  value: string;
  description: string;
}

enum PathKind {
  PATH_KIND_UNSPECIFIED: 0,
  PATH_KIND_INDEX: 1,
  PATH_KIND_INDEX_ANY: 2,
}

@autoFields
model PathKey {
  kind: PathKind;
  key?: string;
  index?: int32;
}

enum AssertKind {
  ASSERT_KIND_UNSPECIFIED: 0,
  ASSERT_KIND_EQUAL: 1,
  ASSERT_KIND_NOT_EQUAL: 2,
  ASSERT_KIND_CONTAINS: 3,
  ASSERT_KIND_NOT_CONTAINS: 4,
  ASSERT_KIND_GREATER: 5,
  ASSERT_KIND_LESS: 6,
  ASSERT_KIND_GREATER_OR_EQUAL: 7,
  ASSERT_KIND_LESS_OR_EQUAL: 8,
}

@autoFields
@parentResource(Example.Example)
model Assert {
  @key assertId: Resource.ID;
  path: PathKey[];
  value: string;
  type: AssertKind;
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
}
