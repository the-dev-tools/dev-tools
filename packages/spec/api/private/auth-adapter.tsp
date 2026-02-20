using DevTools;

namespace Api.Private.AuthAdapter;

enum Operator {
  Equal,
  NotEqual,
  LessThan,
  LessOrEqual,
  GreaterThan,
  GreaterOrEqual,
  In,
  NotIn,
  Contains,
  StartsWith,
  EndsWith,
}

enum Connector {
  And,
  Or,
}

model Where {
  operator: Operator;
  value: Protobuf.WellKnown.Json;
  field: string;
  connector: Connector;
}

enum Direction {
  Ascending,
  Descending,
}

model SortBy {
  field: string;
  direction: Direction;
}

model CreateRequest {
  `model`: string;
  data: Protobuf.Map<string, Protobuf.WellKnown.Json>;
  select?: string[];
}

model CreateResponse {
  data: Protobuf.Map<string, Protobuf.WellKnown.Json>;
}

op Create(...CreateRequest): CreateResponse;

model UpdateRequest {
  `model`: string;
  where: Where[];
  update: Protobuf.WellKnown.Json;
}

model UpdateResponse {
  data?: Protobuf.WellKnown.Json;
}

op Update(...UpdateRequest): UpdateResponse;

model UpdateManyRequest {
  `model`: string;
  where: Where[];
  update: Protobuf.Map<string, Protobuf.WellKnown.Json>;
}

model UpdateManyResponse {
  count: int32;
}

op UpdateMany(...UpdateManyRequest): UpdateManyResponse;

model FindRequest {
  `model`: string;
  where: Where[];
  select?: string[];
}

model FindResponse {
  data?: Protobuf.WellKnown.Json;
}

op Find(...FindRequest): FindResponse;

model FindManyRequest {
  `model`: string;
  where?: Where[];
  limit: int32;
  sortBy?: SortBy;
  offset?: int32;
}

model FindManyResponse {
  items: Protobuf.WellKnown.Json[];
}

op FindMany(...FindManyRequest): FindManyResponse;

model DeleteRequest {
  `model`: string;
  where: Where[];
}

op Delete(...DeleteRequest): {};

model DeleteManyRequest {
  `model`: string;
  where: Where[];
}

model DeleteManyResponse {
  count: int32;
}

op DeleteMany(...DeleteManyRequest): DeleteManyResponse;

model CountRequest {
  `model`: string;
  where?: Where[];
}

model CountResponse {
  count: int32;
}

op Count(...CountRequest): CountResponse;
