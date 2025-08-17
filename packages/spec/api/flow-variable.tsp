using DevTools;

namespace API.FlowVariable;

@DataClient.entity
@parent(Flow.Flow)
model FlowVariable {
  @key variableId: Resource.Id;
  name: string;
  value: string;
  enabled: boolean;
  description: string;
}

model FlowVariableMoveRequest {
  ...ParentKeyOf<FlowVariable>;
  ...KeyOf<FlowVariable>;
  position: Resource.MovePosition;
  targetVariableId: Resource.Id;
}

model FlowVariableMoveResponse {}

interface FlowVariableService extends Resource.CRUD<FlowVariable> {
  @DataClient.endpoint("flow.js:moveVariable")
  FlowVariableMove(...FlowVariableMoveRequest): FlowVariableMoveResponse;
}
