using DevTools;

namespace Api.AuthAdapter;

enum Operator {
  Equal,
  NotEqual,
  LessThan,
  LessOrEqual,
  GreaterThan,
  GreaterOrEqual,
  In,
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
  data: Protobuf.WellKnown.Json;
  select?: string[];
}

model CreateResponse {
  data: Protobuf.WellKnown.Json;
}

model UpdateRequest {
  `model`: string;
  where: Where[];
  update: Protobuf.WellKnown.Json;
}

model UpdateResponse {
  data?: Protobuf.WellKnown.Json;
}

model UpdateManyRequest {
  `model`: string;
  where: Where[];
  update: Protobuf.Map<string, Protobuf.WellKnown.Json>;
}

model UpdateManyResponse {
  count: int32;
}

model FindRequest {
  `model`: string;
  where: Where[];
  select?: string[];
}

model FindResponse {
  data?: Protobuf.WellKnown.Json;
}

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

model DeleteRequest {
  `model`: string;
  where: Where[];
}

model DeleteManyRequest {
  `model`: string;
  where: Where[];
}

model DeleteManyResponse {
  count: int32;
}

model CountRequest {
  `model`: string;
  where?: Where[];
}

model CountResponse {
  count: int32;
}

interface AuthAdapterService {
  op Create(...CreateRequest): CreateResponse;
  op Update(...UpdateRequest): UpdateResponse;
  op UpdateMany(...UpdateManyRequest): UpdateManyResponse;
  op Find(...FindRequest): FindResponse;
  op FindMany(...FindManyRequest): FindManyResponse;
  op Delete(...DeleteRequest): {};
  op DeleteMany(...DeleteManyRequest): DeleteManyResponse;
  op Count(...CountRequest): CountResponse;
}
