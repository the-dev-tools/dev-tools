namespace API.VariableService;

using TypeSpec.Rest;
using TypeSpec.OpenAPI;

@resource("variable")
model Variable {
  @key variableId: string;
  name: string;
  value: string;
  enabled: boolean;
  description: string;
}

@autoRoute
@operationGroup
interface VariableService extends Resource.ResourceOperations<Variable, Error> {}
