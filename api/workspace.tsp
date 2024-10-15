import "@typespec/protobuf";
import "@typespec/rest";

import "./environment.tsp";
import "./resource.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "workspace.v1",
  options: {
    go_package: "dev-tools-services/gen/workspace/v1;workspacev1",
  },
})
namespace API.Workspace;

model Workspace {
  @field(1) @key workspaceId: string;
  @field(2) name: string;
  @field(3) @visibility("query") updated: WellKnown.Timestamp;
  @field(4) @visibility("get") environment: Environment.Environment;
}

enum Role {
  ROLE_UNSPECIFIED: 0,
  ROLE_MEMBER: 1,
  ROLE_ADMIN: 2,
  ROLE_OWNER: 3,
}

@parentResource(Workspace)
model Member {
  @field(2) @key memberId: string;
  @field(3) @visibility("create", "query") email: string;
  @field(4) role: Role;
}

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

@Protobuf.service
interface WorkspaceService {
  ListWorkspaces(...ListWorkspacesRequest): ListWorkspacesResponse;
  GetWorkspace(...GetWorkspaceRequest): GetWorkspaceResponse;
  CreateWorkspace(...CreateWorkspaceRequest): CreateWorkspaceResponse;
  UpdateWorkspace(...UpdateWorkspaceRequest): UpdateWorkspaceResponse;
  DeleteWorkspace(...DeleteWorkspaceRequest): DeleteWorkspaceResponse;
}

model ListMembersItem is Resource.List.Item<Member>;
model ListMembersRequest is Resource.List.Request<Member>;
model ListMembersResponse is Resource.List.Response<ListMembersItem>;
model CreateMemberRequest is Resource.Create.Request<Member>;
model CreateMemberResponse is Resource.Create.Response<Member>;
model UpdateMemberRequest is Resource.Update.Request<Member>;
model UpdateMemberResponse is Resource.Update.Response<Member>;
model DeleteMemberRequest is Resource.Delete.Request<Member>;
model DeleteMemberResponse is Resource.Delete.Response<Member>;

@Protobuf.service
interface MemberService {
  ListMembers(...ListMembersRequest): ListMembersResponse;
  CreateMember(...CreateMemberRequest): CreateMemberResponse;
  UpdateMember(...UpdateMemberRequest): UpdateMemberResponse;
  DeleteMember(...DeleteMemberRequest): DeleteMemberResponse;
}
