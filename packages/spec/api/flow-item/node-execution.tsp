using DevTools;

namespace API.FlowItem.NodeExecution;

@parent(Node.Node)
@DataClient.entity
model NodeExecution {
  @key nodeExecutionId: Resource.Id;
  nodeId: Resource.Id;
  name: string;
  state: Node.NodeState;
  error?: string;
  input?: Protobuf.WellKnown.Json;
  output?: Protobuf.WellKnown.Json;
  responseId?: Resource.Id; // present when execution belongs to a request node
  completedAt?: Protobuf.WellKnown.Timestamp;
}

interface NodeExecutionService extends Resource.List.Interface<NodeExecution>, Resource.Get.Interface<NodeExecution> {}
