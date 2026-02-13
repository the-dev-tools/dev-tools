using DevTools;

namespace Api.GraphQL;

@TanStackDB.collection
model GraphQL {
  @primaryKey graphqlId: Id;
  name: string;
  url: string;
  query: string;
  variables: string;
  lastRunAt?: Protobuf.WellKnown.Timestamp;
}

@TanStackDB.collection
model GraphQLHeader {
  @primaryKey graphqlHeaderId: Id;
  @foreignKey graphqlId: Id;
  key: string;
  value: string;
  enabled: boolean;
  description: string;
  order: float32;
}

@TanStackDB.collection(#{ isReadOnly: true })
model GraphQLResponse {
  @primaryKey graphqlResponseId: Id;
  @foreignKey graphqlId: Id;
  status: int32;
  body: string;
  time: Protobuf.WellKnown.Timestamp;
  duration: int32;
  size: int32;
}

@TanStackDB.collection(#{ isReadOnly: true })
model GraphQLResponseHeader {
  @primaryKey graphqlResponseHeaderId: Id;
  @foreignKey graphqlResponseId: Id;
  key: string;
  value: string;
}

model GraphQLRunRequest {
  graphqlId: Id;
}

op GraphQLRun(...GraphQLRunRequest): {};

model GraphQLDuplicateRequest {
  graphqlId: Id;
}

op GraphQLDuplicate(...GraphQLDuplicateRequest): {};

model GraphQLIntrospectRequest {
  graphqlId: Id;
}

model GraphQLIntrospectResponse {
  sdl: string;
  introspectionJson: string;
}

op GraphQLIntrospect(...GraphQLIntrospectRequest): GraphQLIntrospectResponse;
