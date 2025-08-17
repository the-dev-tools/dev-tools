using DevTools;

namespace API.Workspace;

@DataClient.entity
model Workspace {
  @key workspaceId: Resource.Id;
  selectedEnvironmentId: Resource.Id;
  name: string;
  @visibility(Resource.Mutation.None) updated?: Protobuf.WellKnown.Timestamp;
  @visibility(Resource.Mutation.None) collectionCount: int32;
  @visibility(Resource.Mutation.None) flowCount: int32;
}

model WorkspaceMoveRequest {
  ...KeyOf<Workspace>;
  position: Resource.MovePosition;
  targetWorkspaceId: Resource.Id;
}

model WorkspaceMoveResponse {}

enum MemberRole {
  Basic,
  Admin,
  Owner,
}

@DataClient.entity
@parent(Workspace)
model WorkspaceMember {
  @key userID: Resource.Id;
  @visibility(Resource.Mutation.Create) email: string;
  role: MemberRole;
}

model WorkspaceMemberUpdateRequest is Resource.Update.UpdateRequest<WorkspaceMember> {
  ...ParentKeyOf<WorkspaceMember>;
}
model WorkspaceMemberDeleteRequest is Resource.Delete.DeleteRequest<WorkspaceMember> {
  ...ParentKeyOf<WorkspaceMember>;
}

interface WorkspaceService
  extends Resource.CRUD<Workspace>,
    Resource.List.Interface<WorkspaceMember>,
    Resource.Create.Interface<WorkspaceMember>,
    Resource.Update.Interface<WorkspaceMember>,
    Resource.Delete.Interface<WorkspaceMember> {
  @DataClient.endpoint("workspace.js:move")
  WorkspaceMove(...WorkspaceMoveRequest): WorkspaceMoveResponse;
}
