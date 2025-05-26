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
  @visibility(Resource.Mutation.None) origin?: Query;
  @visibility(Resource.Mutation.Update) source?: Delta.SourceKind;
  key: string;
  enabled: boolean;
  value: string;
  description: string;
}

@autoFields
@normalize
@entity
@parentResource(Example.Example)
model Header {
  @key headerId: Resource.Id;
  @visibility(Resource.Mutation.None) origin?: Header;
  @visibility(Resource.Mutation.Update) source?: Delta.SourceKind;
  key: string;
  enabled: boolean;
  value: string;
  description: string;
}

@autoFields
@normalize
@entity
@parentResource(Example.Example)
model Assert {
  @key assertId: Resource.Id;
  @visibility(Resource.Mutation.None) origin?: Assert;
  @visibility(Resource.Mutation.Update) source?: Delta.SourceKind;
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
    // Header
    Resource.List.Interface<Header>,
    Resource.Create.Interface<Header>,
    Resource.Update.Interface<Header>,
    Resource.Delete.Interface<Header>,
    // Assert
    Resource.List.Interface<Assert>,
    Resource.Create.Interface<Assert>,
    Resource.Update.Interface<Assert>,
    Resource.Delete.Interface<Assert> {}
