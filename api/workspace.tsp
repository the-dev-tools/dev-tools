using TypeSpec.Protobuf;
using TypeSpec.Rest;

@package({
  name: "workspace",
  options: {
    go_package: "dev-tools-services/gen/workspace",
  },
})
namespace API.Workspace;

model Workspace {
  @field(1) @key workspaceId: string;
  @field(2) name: string;
  @field(3) @visibility("query") updated: WellKnown.Timestamp;

  /** TODO: define proper model */
  @field(4) @visibility("get") environment: bytes;
}

enum Role {
  Member: 0,
  Admin: 1,
  Owner: 2,
}

@parentResource(Workspace)
model Member {
  @field(2) @key memberId: string;
  @field(3) @visibility("create", "query") email: string;
  @field(4) role: Role;
}

model ListWorkspacesItem is Resource.List.Item<Workspace>;
model ListWorkspacesRequest is Resource.List.Request<Workspace>;
model ListWorkspacesResponse is Resource.List.Response<Workspace, ListWorkspacesItem>;
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
  List(...ListWorkspacesRequest): ListWorkspacesResponse;
  Get(...GetWorkspaceRequest): GetWorkspaceResponse;
  Create(...CreateWorkspaceRequest): CreateWorkspaceResponse;
  Update(...UpdateWorkspaceRequest): UpdateWorkspaceResponse;
  Delete(...DeleteWorkspaceRequest): DeleteWorkspaceResponse;
}

model ListMembersItem is Resource.List.Item<Member>;
model ListMembersRequest is Resource.List.Request<Member>;
model ListMembersResponse is Resource.List.Response<Member, ListMembersItem>;
model CreateMemberRequest is Resource.Create.Request<Member>;
model CreateMemberResponse is Resource.Create.Response<Member>;
model UpdateMemberRequest is Resource.Update.Request<Member>;
model UpdateMemberResponse is Resource.Update.Response<Member>;
model DeleteMemberRequest is Resource.Delete.Request<Member>;
model DeleteMemberResponse is Resource.Delete.Response<Member>;

@Protobuf.service
interface MemberService {
  List(...ListMembersRequest): ListMembersResponse;
  Create(...CreateMemberRequest): CreateMemberResponse;
  Update(...UpdateMemberRequest): UpdateMemberResponse;
  Delete(...DeleteMemberRequest): DeleteMemberResponse;
}
