import "@typespec/protobuf";
import "@typespec/rest";

import "../lib";

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
model Workspace {
  @key workspaceId: Resource.Id;
  selectedEnvironmentId: Resource.Id;
  name: string;
  @visibility("query") updated: WellKnown.Timestamp;
}

enum MemberRole {
  MEMBER_ROLE_UNSPECIFIED: 0,
  MEMBER_ROLE_BASIC: 1,
  MEMBER_ROLE_ADMIN: 2,
  MEMBER_ROLE_OWNER: 3,
}

@autoFields
@normalize
@parentResource(Workspace)
model WorkspaceMember {
  @key memberId: Resource.Id;
  @visibility("create", "query") email: string;
  role: MemberRole;
}

// Workspace
model WorkspaceChange is Resource.Change<Workspace>;
model WorkspaceListItem is Resource.List.Item<Workspace>;
model WorkspaceListRequest is Resource.List.Request<Workspace>;
model WorkspaceListResponse is Resource.List.Response<Workspace, WorkspaceListItem>;
model WorkspaceGetRequest is Resource.Get.Request<Workspace>;
model WorkspaceGetResponse is Resource.Get.Response<Workspace>;
model WorkspaceCreateRequest is Resource.Create.Request<Workspace>;
model WorkspaceCreateResponse is Resource.Create.Response<Workspace, WorkspaceListResponse>;
model WorkspaceUpdateRequest is Resource.Update.Request<Workspace>;
model WorkspaceUpdateResponse is Resource.Update.Response<Workspace>;
model WorkspaceDeleteRequest is Resource.Delete.Request<Workspace>;
model WorkspaceDeleteResponse is Resource.Delete.Response<Workspace>;

// Member
model WorkspaceMemberChange is Resource.Change<WorkspaceMember>;
model WorkspaceMemberListItem is Resource.List.Item<WorkspaceMember>;
model WorkspaceMemberListRequest is Resource.List.Request<WorkspaceMember>;
model WorkspaceMemberListResponse is Resource.List.Response<WorkspaceMember, WorkspaceMemberListItem>;
model WorkspaceMemberCreateRequest is Resource.Create.Request<WorkspaceMember>;
model WorkspaceMemberCreateResponse is Resource.Create.Response<WorkspaceMember, WorkspaceMemberListResponse>;
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
