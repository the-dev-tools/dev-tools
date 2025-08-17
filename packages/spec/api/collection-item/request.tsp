using DevTools;

namespace API.CollectionItem.Request;

@DataClient.entity
@parent(Example.Example)
model Query {
  @key queryId: Resource.Id;
  key: string;
  enabled: boolean;
  value: string;
  description: string;
}

model QueryMoveRequest {
  ...ParentKeyOf<Query>;
  ...KeyOf<Query>;
  position: Resource.MovePosition;
  targetQueryId: Resource.Id;
}

model QueryDeltaMoveRequest {
  ...ParentKeyOf<Query>;
  ...KeyOf<Query>;
  originId: Resource.Id;
  position: Resource.MovePosition;
  targetQueryId: Resource.Id;
}

@DataClient.entity
@parent(Example.Example)
model Header {
  @key headerId: Resource.Id;
  key: string;
  enabled: boolean;
  value: string;
  description: string;
}

model HeaderMoveRequest {
  ...ParentKeyOf<Header>;
  ...KeyOf<Header>;
  position: Resource.MovePosition;
  targetHeaderId: Resource.Id;
}

model HeaderDeltaMoveRequest {
  ...ParentKeyOf<Header>;
  ...KeyOf<Header>;
  originId: Resource.Id;
  position: Resource.MovePosition;
  targetHeaderId: Resource.Id;
}

@DataClient.entity
@parent(Example.Example)
model Assert {
  @key assertId: Resource.Id;
  condition: Condition.Condition;
}

interface RequestService
  extends BlankInterface,
    // Query
    Resource.List.Interface<Query>,
    Resource.Create.Interface<Query>,
    Resource.Update.Interface<Query>,
    Resource.Delete.Interface<Query>,
    Delta.Interface<Query>,
    // Header
    Resource.List.Interface<Header>,
    Resource.Create.Interface<Header>,
    Resource.Update.Interface<Header>,
    Resource.Delete.Interface<Header>,
    Delta.Interface<Header>,
    // Assert
    Resource.List.Interface<Assert>,
    Resource.Create.Interface<Assert>,
    Resource.Update.Interface<Assert>,
    Resource.Delete.Interface<Assert>,
    Delta.Interface<Assert> {
  @DataClient.endpoint("request.js:moveQuery")
  QueryMove(...QueryMoveRequest): {};
  @DataClient.endpoint("request.js:moveQueryDelta")
  QueryDeltaMove(...QueryDeltaMoveRequest): {};
  @DataClient.endpoint("request.js:moveHeader")
  HeaderMove(...HeaderMoveRequest): {};
  @DataClient.endpoint("request.js:moveHeaderDelta")
  HeaderDeltaMove(...HeaderDeltaMoveRequest): {};
}
