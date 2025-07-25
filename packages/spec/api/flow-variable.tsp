import "@typespec/protobuf";

import "./lib.tsp";

import "./resource.tsp";
import "./flow.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "flowvariable.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/flowvariable/v1;flowvariablev1",
  },
})
namespace API.FlowVariable;

@autoFields
@normalize
@entity
@parentResource(Flow.Flow)
model FlowVariable {
  @key variableId: Resource.Id;
  name: string;
  value: string;
  enabled: boolean;
  description: string;
}

@autoFields
model FlowVariableMoveRequest {
  ...ParentKeyOf<FlowVariable>;
  ...KeyOf<FlowVariable>;
  position: Resource.MovePosition;
  targetVariableId: Resource.Id;
}

model FlowVariableMoveResponse {}

@Protobuf.service
interface FlowVariableService extends Resource.CRUD<FlowVariable> {
  @endpoint("flow.js:moveVariable")
  FlowVariableMove(...FlowVariableMoveRequest): FlowVariableMoveResponse;
}
