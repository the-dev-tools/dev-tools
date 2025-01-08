import "@typespec/protobuf";
import "@typespec/rest";

import "../../lib";

import "../assert.tsp";
import "../reference.tsp";
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
  @key queryId: Resource.Id;
  @visibility("query") parentQueryId?: Resource.Id;
  key: string;
  enabled: boolean;
  value: string;
  description: string;
}

@autoFields
@parentResource(Example.Example)
model Header {
  @key headerId: Resource.Id;
  @visibility("query") parentHeaderId?: Resource.Id;
  key: string;
  enabled: boolean;
  value: string;
  description: string;
}

@autoFields
@parentResource(Example.Example)
model Assert {
  @key assertId: Resource.Id;
  @visibility("query") parentAssertId?: Resource.Id;
  path: Reference.ReferenceKey[];
  value: string;
  type: API.Assert.AssertKind;
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
