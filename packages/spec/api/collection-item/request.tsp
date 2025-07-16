import "@typespec/protobuf";
import "@typespec/rest";

import "../lib.tsp";

import "../delta.tsp";
import "../condition.tsp";
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
@entity
@parentResource(Example.Example)
model Query {
  @key queryId: Resource.Id;
  key: string;
  enabled: boolean;
  value: string;
  description: string;
}

model QueryDelta is Delta.Item<Query>;

@autoFields
model QueryMoveRequest {
  ...ParentKeyOf<Query>;
  ...KeyOf<Query>;
  position: Resource.MovePosition;
  targetQueryId: Resource.Id;
}

model QueryMoveResponse {}

@autoFields
model QueryDeltaMoveRequest {
  ...ParentKeyOf<QueryDelta>;
  ...KeyOf<QueryDelta>;
  originId: Resource.Id;
  position: Resource.MovePosition;
  targetQueryId: Resource.Id;
}

model QueryDeltaMoveResponse {}

@autoFields
@normalize
@entity
@parentResource(Example.Example)
model Header {
  @key headerId: Resource.Id;
  key: string;
  enabled: boolean;
  value: string;
  description: string;
}

model HeaderDelta is Delta.Item<Header>;

@autoFields
model HeaderMoveRequest {
  ...ParentKeyOf<Header>;
  ...KeyOf<Header>;
  position: Resource.MovePosition;
  targetHeaderId: Resource.Id;
}

model HeaderMoveResponse {}

@autoFields
model HeaderDeltaMoveRequest {
  ...ParentKeyOf<HeaderDelta>;
  ...KeyOf<HeaderDelta>;
  originId: Resource.Id;
  position: Resource.MovePosition;
  targetHeaderId: Resource.Id;
}

model HeaderDeltaMoveResponse {}

@autoFields
@normalize
@entity
@parentResource(Example.Example)
model Assert {
  @key assertId: Resource.Id;
  condition: Condition.Condition;
}

@Protobuf.service
interface RequestService
  extends BlankInterface,
    // Query
    Resource.List.Interface<Query>,
    Resource.Create.Interface<Query>,
    Resource.Update.Interface<Query>,
    Resource.Delete.Interface<Query>,
    Delta.Interface<Query, TDelta = QueryDelta>,
    // Header
    Resource.List.Interface<Header>,
    Resource.Create.Interface<Header>,
    Resource.Update.Interface<Header>,
    Resource.Delete.Interface<Header>,
    Delta.Interface<Header, TDelta = HeaderDelta>,
    // Assert
    Resource.List.Interface<Assert>,
    Resource.Create.Interface<Assert>,
    Resource.Update.Interface<Assert>,
    Resource.Delete.Interface<Assert>,
    Delta.Interface<Assert> {
  @endpoint("request.js:moveQuery")
  QueryMove(...QueryMoveRequest): QueryMoveResponse;
  @endpoint("request.js:moveQueryDelta")
  QueryDeltaMove(...QueryDeltaMoveRequest): QueryDeltaMoveResponse;
  @endpoint("request.js:moveHeader")
  HeaderMove(...HeaderMoveRequest): HeaderMoveResponse;
  @endpoint("request.js:moveHeaderDelta")
  HeaderDeltaMove(...HeaderDeltaMoveRequest): HeaderDeltaMoveResponse;
}
