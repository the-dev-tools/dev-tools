using DevTools;

namespace API.Workspace;

@DataClient.entity
model Workspace {
  @key workspaceId: Resource.Id;
  selectedEnvironmentId: Resource.Id;
  name: string;
  @visibility(T.Resource.Mutation.None) updated?: Protobuf.WellKnown.Timestamp;
  @visibility(T.Resource.Mutation.None) collectionCount: int32;
  @visibility(T.Resource.Mutation.None) flowCount: int32;
}

model WorkspaceListRequest is T.Resource.List.Request<Workspace>;
model WorkspaceListResponse is T.Resource.List.Response<TRequest = WorkspaceListRequest, TItem = WorkspaceListItem>;
model WorkspaceListItem is T.Resource.List.Item<Workspace>;

model WorkspaceGetRequest is T.Resource.Get.Request<Workspace>;
model WorkspaceGetResponse is T.Resource.Get.Response<Workspace>;

model WorkspaceCreateRequest is T.Resource.Create.Request<Workspace>;
model WorkspaceCreateResponse is T.Resource.Create.Response<Workspace>;

model WorkspaceUpdateRequest is T.Resource.Update.Request<Workspace>;
model WorkspaceUpdateResponse is T.Resource.Update.Response<Workspace>;

model WorkspaceDeleteRequest is T.Resource.Delete.Request<Workspace>;
model WorkspaceDeleteResponse is T.Resource.Delete.Response<Workspace>;

model WorkspaceMoveRequest {
  ...KeyOf<Workspace>;
  position: Resource.MovePosition;
  targetWorkspaceId: Resource.Id;
}

enum MemberRole {
  Basic,
  Admin,
  Owner,
}

@DataClient.entity
@parent(Workspace)
model WorkspaceMember {
  @key userID: Resource.Id;
  @visibility(T.Resource.Mutation.Create) email: string;
  role: MemberRole;
}

model WorkspaceMemberListRequest is T.Resource.List.Request<WorkspaceMember>;
model WorkspaceMemberListResponse
  is T.Resource.List.Response<TRequest = WorkspaceMemberListRequest, TItem = WorkspaceMemberListItem>;
model WorkspaceMemberListItem is T.Resource.List.Item<WorkspaceMember>;

model WorkspaceMemberGetRequest is T.Resource.Get.Request<WorkspaceMember>;
model WorkspaceMemberGetResponse is T.Resource.Get.Response<WorkspaceMember>;

model WorkspaceMemberCreateRequest is T.Resource.Create.Request<WorkspaceMember>;
model WorkspaceMemberCreateResponse is T.Resource.Create.Response<WorkspaceMember>;

model WorkspaceMemberUpdateRequest is T.Resource.Update.Request<WorkspaceMember> {
  ...ParentKeyOf<WorkspaceMember>;
}
model WorkspaceMemberUpdateResponse is T.Resource.Update.Response<WorkspaceMember>;

model WorkspaceMemberDeleteRequest is T.Resource.Delete.Request<WorkspaceMember> {
  ...ParentKeyOf<WorkspaceMember>;
}
model WorkspaceMemberDeleteResponse is T.Resource.Delete.Response<WorkspaceMember>;

interface WorkspaceService
  extends T.Resource.CRUD<
      TResource = Workspace,
      // List
      TListRequest = WorkspaceListRequest,
      TListResponse = WorkspaceListResponse,
      TListItem = WorkspaceListItem,
      // Get
      TGetRequest = WorkspaceGetRequest,
      TGetResponse = WorkspaceGetResponse,
      // Create
      TCreateRequest = WorkspaceCreateRequest,
      TCreateResponse = WorkspaceCreateResponse,
      // Update
      TUpdateRequest = WorkspaceUpdateRequest,
      TUpdateResponse = WorkspaceUpdateResponse,
      // Delete
      TDeleteRequest = WorkspaceDeleteRequest,
      TDeleteResponse = WorkspaceDeleteResponse
    >,
    T.Resource.List.Interface<
      TResource = WorkspaceMember,
      TRequest = WorkspaceMemberListRequest,
      TResponse = WorkspaceMemberListResponse,
      TItem = WorkspaceMemberListItem
    >,
    T.Resource.Create.Interface<
      TResource = WorkspaceMember,
      TRequest = WorkspaceMemberCreateRequest,
      TResponse = WorkspaceMemberCreateResponse,
      TListRequest = WorkspaceMemberListRequest,
      TListItem = WorkspaceMemberListItem
    >,
    T.Resource.Update.Interface<
      TResource = WorkspaceMember,
      TRequest = WorkspaceMemberUpdateRequest,
      TResponse = WorkspaceMemberUpdateResponse
    >,
    T.Resource.Delete.Interface<
      TResource = WorkspaceMember,
      TRequest = WorkspaceMemberDeleteRequest,
      TResponse = WorkspaceMemberDeleteResponse
    > {
  @DataClient.endpoint("workspace.js:move")
  WorkspaceMove(...WorkspaceMoveRequest): {};
}
