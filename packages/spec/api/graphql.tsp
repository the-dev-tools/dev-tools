using DevTools;

namespace Api.GraphQL;

@withDelta
@TanStackDB.collection
model GraphQL {
  @primaryKey graphqlId: Id;
  name: string;
  url: string;
  query: string;
  variables: string;
  lastRunAt?: Protobuf.WellKnown.Timestamp;
}

@TanStackDB.collection(#{ isReadOnly: true })
model GraphQLVersion {
  @primaryKey graphqlVersionId: Id;
  @foreignKey graphqlId: Id;
  @foreignKey deltaGraphqlId?: Id;
  name: string;
  description: string;
  createdAt: int64;
}

@withDelta
@TanStackDB.collection
model GraphQLHeader {
  @primaryKey graphqlHeaderId: Id;
  ...CommonTableFields<GraphQL>;
}

@withDelta
@TanStackDB.collection
model GraphQLAssert {
  @primaryKey graphqlAssertId: Id;
  @foreignKey graphqlId: Id;
  value: string;
  enabled: boolean;
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

@TanStackDB.collection(#{ isReadOnly: true })
model GraphQLResponseAssert {
  @primaryKey graphqlResponseAssertId: Id;
  @foreignKey graphqlResponseId: Id;
  value: string;
  success: boolean;
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
