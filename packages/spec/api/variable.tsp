using DevTools;

namespace API.Variable;

@DataClient.entity
@parent(Environment.Environment)
model Variable {
  @key variableId: Resource.Id;
  name: string;
  value: string;
  enabled: boolean;
  description: string;
}

model VariableCreateRequest is Resource.Create.CreateRequest<Variable> {
  @normalKey workspaceId?: Resource.Id;
}
model VariableListRequest is Resource.List.ListRequest<Variable> {
  @normalKey workspaceId?: Resource.Id;
}

model VariableMoveRequest {
  ...ParentKeyOf<Variable>;
  ...KeyOf<Variable>;
  position: Resource.MovePosition;
  targetVariableId: Resource.Id;
}

model VariableMoveResponse {}

interface VariableService extends Resource.CRUD<Variable> {
  @DataClient.endpoint("variable.js:move")
  VariableMove(...VariableMoveRequest): VariableMoveResponse;
}
