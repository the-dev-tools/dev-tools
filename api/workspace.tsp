namespace API.WorkspaceService;

using TypeSpec.Rest;
using TypeSpec.OpenAPI;

@resource("workspace")
model Workspace {
  @key workspaceId: string;
  name: string;
  updated: utcDateTime;

  /** TODO: define proper model */
  environment: bytes;
}

enum Role {
  Member: 0,
  Admin: 1,
  Owner: 2,
}

@resource("user")
@parentResource(Workspace)
model WorkspaceUser {
  @key userId: string;
  @visibility("create", "query") email: string;
  role: Role;
}

@autoRoute
@operationGroup
interface WorkspaceService extends Resource.ResourceOperations<Workspace, Error> {}

@autoRoute
@operationGroup
interface WorkspaceUserService extends Resource.ResourceOperations<WorkspaceUser, Error> {}
