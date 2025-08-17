using DevTools;

namespace API.FlowItem.Edge;

enum Handle {
  Then,
  Else,
  Loop,
}

enum EdgeKind {
  NoOp,
}

@parent(Flow.Flow)
@DataClient.entity
model Edge {
  @key edgeId: Resource.Id;
  kind: EdgeKind;
  sourceId: Resource.Id;
  targetId: Resource.Id;
  sourceHandle: Handle;
}

interface EdgeService extends Resource.CRUD<Edge> {}
