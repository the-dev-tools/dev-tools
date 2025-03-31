import "@typespec/protobuf";
import "@typespec/rest";

import "../../lib";

import "../condition.tsp";
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
@normalize
@parentResource(Example.Example)
model Query {
  @key queryId: Resource.Id;
  @visibility(Resource.Mutation.Create) parentQueryId?: Resource.Id;
  key: string;
  enabled: boolean;
  value: string;
  description: string;
}

@autoFields
@normalize
@parentResource(Example.Example)
model Header {
  @key headerId: Resource.Id;
  @visibility(Resource.Mutation.Create) parentHeaderId?: Resource.Id;
  key: string;
  enabled: boolean;
  value: string;
  description: string;
}

@autoFields
@normalize
@parentResource(Example.Example)
model Assert {
  @key assertId: Resource.Id;
  @visibility(Resource.Mutation.Create) parentAssertId?: Resource.Id;
  condition: Condition.Condition;
}

// Query
model QueryChange is Resource.Change<Query>;
model QueryListItem is Resource.List.Item<Query>;
model QueryListRequest is Resource.List.Request<Query>;
model QueryListResponse is Resource.List.Response<Query, QueryListItem>;
model QueryCreateRequest is Resource.Create.Request<Query>;
model QueryCreateResponse is Resource.Create.Response<Query, QueryListResponse>;
model QueryUpdateRequest is Resource.Update.Request<Query>;
model QueryUpdateResponse is Resource.Update.Response<Query>;
model QueryDeleteRequest is Resource.Delete.Request<Query>;
model QueryDeleteResponse is Resource.Delete.Response<Query>;

// Header
model HeaderChange is Resource.Change<Header>;
model HeaderListItem is Resource.List.Item<Header>;
model HeaderListRequest is Resource.List.Request<Header>;
model HeaderListResponse is Resource.List.Response<Header, HeaderListItem>;
model HeaderCreateRequest is Resource.Create.Request<Header>;
model HeaderCreateResponse is Resource.Create.Response<Header, HeaderListResponse>;
model HeaderUpdateRequest is Resource.Update.Request<Header>;
model HeaderUpdateResponse is Resource.Update.Response<Header>;
model HeaderDeleteRequest is Resource.Delete.Request<Header>;
model HeaderDeleteResponse is Resource.Delete.Response<Header>;

// Assert
model AssertChange is Resource.Change<Assert>;
model AssertListItem is Resource.List.Item<Assert>;
model AssertListRequest is Resource.List.Request<Assert>;
model AssertListResponse is Resource.List.Response<Assert, AssertListItem>;
model AssertCreateRequest is Resource.Create.Request<Assert>;
model AssertCreateResponse is Resource.Create.Response<Assert, AssertListResponse>;
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
