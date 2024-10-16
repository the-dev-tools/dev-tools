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
  @field(3) name: string;
  @field(4) @visibility("query") updated: WellKnown.Timestamp;
  @field(5) @visibility("get") environment: Environment.Environment;
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
model WorkspaceListItem is Resource.List.Item<Workspace>;
model WorkspaceListRequest is Resource.List.Request<Workspace>;
model WorkspaceListResponse is Resource.List.Response<WorkspaceListItem>;
model WorkspaceGetRequest is Resource.Get.Request<Workspace>;
model WorkspaceGetResponse is Resource.Get.Response<Workspace>;
model WorkspaceCreateRequest is Resource.Create.Request<Workspace>;
model WorkspaceCreateResponse is Resource.Create.Response<Workspace>;
model WorkspaceUpdateRequest is Resource.Update.Request<Workspace> {
  ...KeyOf<Environment.Environment>;
}
model WorkspaceUpdateResponse is Resource.Update.Response<Workspace>;
model WorkspaceDeleteRequest is Resource.Delete.Request<Workspace>;
model WorkspaceDeleteResponse is Resource.Delete.Response<Workspace>;

// Member
model WorkspaceMemberListItem is Resource.List.Item<WorkspaceMember>;
model WorkspaceMemberListRequest is Resource.List.Request<WorkspaceMember>;
model WorkspaceMemberListResponse is Resource.List.Response<WorkspaceMemberListItem>;
model WorkspaceMemberCreateRequest is Resource.Create.Request<WorkspaceMember>;
model WorkspaceMemberCreateResponse is Resource.Create.Response<WorkspaceMember>;
model WorkspaceMemberUpdateRequest is Resource.Update.Request<WorkspaceMember>;
model WorkspaceMemberUpdateResponse is Resource.Update.Response<WorkspaceMember>;
model WorkspaceMemberDeleteRequest is Resource.Delete.Request<WorkspaceMember>;
model WorkspaceMemberDeleteResponse is Resource.Delete.Response<WorkspaceMember>;

@Protobuf.service
interface WorkspaceService {
  // Workspace
  WorkspaceList(...WorkspaceListRequest): WorkspaceListResponse;
  WorkspaceGet(...WorkspaceGetRequest): WorkspaceGetResponse;
  WorkspaceCreate(...WorkspaceCreateRequest): WorkspaceCreateResponse;
  WorkspaceUpdate(...WorkspaceUpdateRequest): WorkspaceUpdateResponse;
  WorkspaceDelete(...WorkspaceDeleteRequest): WorkspaceDeleteResponse;

  // Member
  WorkspaceMemberList(...WorkspaceMemberListRequest): WorkspaceMemberListResponse;
  WorkspaceMemberCreate(...WorkspaceMemberCreateRequest): WorkspaceMemberCreateResponse;
  WorkspaceMemberUpdate(...WorkspaceMemberUpdateRequest): WorkspaceMemberUpdateResponse;
  WorkspaceMemberDelete(...WorkspaceMemberDeleteRequest): WorkspaceMemberDeleteResponse;
}
