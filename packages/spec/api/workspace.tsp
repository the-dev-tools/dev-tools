import "@typespec/protobuf";
import "@typespec/rest";

import "./lib.tsp";

import "./environment.tsp";
import "./resource.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "workspace.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/workspace/v1;workspacev1",
  },
})
namespace API.Workspace;

@autoFields
@normalize
@entity
model Workspace {
  @key workspaceId: Resource.Id;
  selectedEnvironmentId: Resource.Id;
  name: string;
  @visibility(Resource.Mutation.None) updated: WellKnown.Timestamp;
  @visibility(Resource.Mutation.None) collectionCount: int32;
  @visibility(Resource.Mutation.None) flowCount: int32;
}

@autoFields
model WorkspaceMoveRequest {
  ...KeyOf<Workspace>;
  position: Resource.MovePosition;
  targetWorkspaceId: Resource.Id;
}

model WorkspaceMoveResponse {}

enum MemberRole {
  MEMBER_ROLE_UNSPECIFIED: 0,
  MEMBER_ROLE_BASIC: 1,
  MEMBER_ROLE_ADMIN: 2,
  MEMBER_ROLE_OWNER: 3,
}

@autoFields
@normalize
@entity
@parentResource(Workspace)
model WorkspaceMember {
  @key userID: Resource.Id;
  @visibility(Resource.Mutation.Create) email: string;
  role: MemberRole;
}

model WorkspaceMemberUpdateRequest is Resource.Update.Request<WorkspaceMember> {
  ...ParentKeyOf<WorkspaceMember>;
}
model WorkspaceMemberDeleteRequest is Resource.Delete.Request<WorkspaceMember> {
  ...ParentKeyOf<WorkspaceMember>;
}

@Protobuf.service
interface WorkspaceService
  extends Resource.CRUD<Workspace>,
    Resource.List.Interface<WorkspaceMember>,
    Resource.Create.Interface<WorkspaceMember>,
    Resource.Update.Interface<WorkspaceMember, TRequest = WorkspaceMemberUpdateRequest>,
    Resource.Delete.Interface<WorkspaceMember, TRequest = WorkspaceMemberDeleteRequest> {
  @endpoint("workspace.js:move")
  WorkspaceMove(...WorkspaceMoveRequest): WorkspaceMoveResponse;
}
