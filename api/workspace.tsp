import "@typespec/protobuf";
import "@typespec/rest";

import "./environment.tsp";
import "./resource.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "workspace.v1",
  options: {
    go_package: "dev-tools-spec/dist/buf/go/workspace/v1;workspacev1",
  },
})
namespace API.Workspace;

model Workspace {
  @field(1) @key workspaceId: Resource.ID;
  @field(2) name: string;
  @field(3) @visibility("query") updated: WellKnown.Timestamp;
  @field(4) @visibility("get") environment: Environment.Environment;
}

enum MemberRole {
  MEMBER_ROLE_UNSPECIFIED: 0,
  MEMBER_ROLE_BASIC: 1,
  MEMBER_ROLE_ADMIN: 2,
  MEMBER_ROLE_OWNER: 3,
}

@parentResource(Workspace)
model WorkspaceMember {
  @field(2) @key memberId: Resource.ID;
  @field(3) @visibility("create", "query") email: string;
  @field(4) role: MemberRole;
}

// Workspace
model ListWorkspacesItem is Resource.List.Item<Workspace>;
model ListWorkspacesRequest is Resource.List.Request<Workspace>;
model ListWorkspacesResponse is Resource.List.Response<ListWorkspacesItem>;
model GetWorkspaceRequest is Resource.Get.Request<Workspace>;
model GetWorkspaceResponse is Resource.Get.Response<Workspace>;
model CreateWorkspaceRequest is Resource.Create.Request<Workspace>;
model CreateWorkspaceResponse is Resource.Create.Response<Workspace>;
model UpdateWorkspaceRequest is Resource.Update.Request<Workspace>;
model UpdateWorkspaceResponse is Resource.Update.Response<Workspace>;
model DeleteWorkspaceRequest is Resource.Delete.Request<Workspace>;
model DeleteWorkspaceResponse is Resource.Delete.Response<Workspace>;

model SetWorkspaceEnvironmentRequest {
  ...KeyOf<Workspace>;
  ...KeyOf<Environment.Environment>;
}
model SetWorkspaceEnvironmentResponse {}

// Member
model ListWorkspaceMembersItem is Resource.List.Item<WorkspaceMember>;
model ListWorkspaceMembersRequest is Resource.List.Request<WorkspaceMember>;
model ListWorkspaceMembersResponse is Resource.List.Response<ListWorkspaceMembersItem>;
model CreateWorkspaceMemberRequest is Resource.Create.Request<WorkspaceMember>;
model CreateWorkspaceMemberResponse is Resource.Create.Response<WorkspaceMember>;
model UpdateWorkspaceMemberRequest is Resource.Update.Request<WorkspaceMember>;
model UpdateWorkspaceMemberResponse is Resource.Update.Response<WorkspaceMember>;
model DeleteWorkspaceMemberRequest is Resource.Delete.Request<WorkspaceMember>;
model DeleteWorkspaceMemberResponse is Resource.Delete.Response<WorkspaceMember>;

@Protobuf.service
interface WorkspaceService {
  // Workspace
  ListWorkspaces(...ListWorkspacesRequest): ListWorkspacesResponse;
  GetWorkspace(...GetWorkspaceRequest): GetWorkspaceResponse;
  CreateWorkspace(...CreateWorkspaceRequest): CreateWorkspaceResponse;
  UpdateWorkspace(...UpdateWorkspaceRequest): UpdateWorkspaceResponse;
  DeleteWorkspace(...DeleteWorkspaceRequest): DeleteWorkspaceResponse;

  SetWorkspaceEnvironment(...SetWorkspaceEnvironmentRequest): SetWorkspaceEnvironmentResponse;

  // Member
  ListWorkspaceMembers(...ListWorkspaceMembersRequest): ListWorkspaceMembersResponse;
  CreateWorkspaceMember(...CreateWorkspaceMemberRequest): CreateWorkspaceMemberResponse;
  UpdateWorkspaceMember(...UpdateWorkspaceMemberRequest): UpdateWorkspaceMemberResponse;
  DeleteWorkspaceMember(...DeleteWorkspaceMemberRequest): DeleteWorkspaceMemberResponse;
}
