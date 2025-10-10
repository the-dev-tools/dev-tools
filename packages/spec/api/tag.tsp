using DevTools;

namespace API.Tag;

enum TagColor {
  Slate,
  Green,
  Amber,
  Sky,
  Purple,
  Rose,
  Blue,
  Fuchsia,
}

@DataClient.entity
@parent(Workspace.Workspace)
model Tag {
  @key tagId: Resource.Id;
  name: string;
  color: TagColor;
}

model TagListRequest is T.Resource.List.Request<Tag>;
model TagListResponse is T.Resource.List.Response<TRequest = TagListRequest, TItem = TagListItem>;
model TagListItem is T.Resource.List.Item<Tag>;

model TagGetRequest is T.Resource.Get.Request<Tag>;
model TagGetResponse is T.Resource.Get.Response<Tag>;

model TagCreateRequest is T.Resource.Create.Request<Tag>;
model TagCreateResponse is T.Resource.Create.Response<Tag>;

model TagUpdateRequest is T.Resource.Update.Request<Tag>;
model TagUpdateResponse is T.Resource.Update.Response<Tag>;

model TagDeleteRequest is T.Resource.Delete.Request<Tag>;
model TagDeleteResponse is T.Resource.Delete.Response<Tag>;

interface TagService
  extends T.Resource.CRUD<
      TResource = Tag,
      // List
      TListRequest = TagListRequest,
      TListResponse = TagListResponse,
      TListItem = TagListItem,
      // Get
      TGetRequest = TagGetRequest,
      TGetResponse = TagGetResponse,
      // Create
      TCreateRequest = TagCreateRequest,
      TCreateResponse = TagCreateResponse,
      // Update
      TUpdateRequest = TagUpdateRequest,
      TUpdateResponse = TagUpdateResponse,
      // Delete
      TDeleteRequest = TagDeleteRequest,
      TDeleteResponse = TagDeleteResponse
    > {}
