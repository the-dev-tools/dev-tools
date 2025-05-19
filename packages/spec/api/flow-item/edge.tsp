import "@typespec/protobuf";
import "@typespec/rest";

import "../lib.tsp";

import "../resource.tsp";
import "../flow.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "flow.edge.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/flow/edge/v1;edgev1",
  },
})
namespace API.FlowItem.Edge;

enum Handle {
  HANDLE_UNSPECIFIED: 0,
  HANDLE_THEN: 1,
  HANDLE_ELSE: 2,
  HANDLE_LOOP: 3,
}

@parentResource(Flow.Flow)
@normalize
@entity
@autoFields
model Edge {
  @key edgeId: Resource.Id;
  sourceId: Resource.Id;
  targetId: Resource.Id;
  sourceHandle: Handle;
}

@Protobuf.service
interface EdgeService extends Resource.CRUD<Edge> {}
