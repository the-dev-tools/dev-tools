using DevTools;
using TypeSpec.Reflection;

@DevTools.project
namespace API.Delta {
  enum SourceKind {
    Origin,
    Mixed,
    Delta,
  }
}

namespace T.Delta {
  using API;
  using API.Delta;

  @DataClient.entity
  @copyParent(TResource)
  model Item<TResource extends Model> {
    ...TResource;
    @visibility(T.Resource.Mutation.None) origin?: TResource;
    @visibility(T.Resource.Mutation.None) source?: SourceKind;
  }

  namespace List {
    model Request<TResource extends Model> is T.Resource.List.Request<TResource> {
      @normalKey originId: Resource.Id;
    }

    model Response<TRequest extends Model, TItem extends Model>
      is T.Resource.List.Response<TRequest = TRequest, TItem = TItem>;

    model Item<TResource extends Model> is T.Resource.List.Item<TResource>;
  }

  namespace Create {
    model Request<TResource extends Model> is T.Resource.Create.Request<TResource> {
      @normalKey originId: Resource.Id;
    }

    model Response<TResource extends Model> is T.Resource.Create.Response<TResource>;
  }

  namespace Update {
    model Request<TResource extends Model> is T.Resource.Update.Request<TResource> {
      ...ParentKeyOf<TResource>;
    }

    model Response<TResource extends Model> is T.Resource.Update.Response<TResource>;
  }

  namespace Delete {
    model Request<TResource extends Model> is T.Resource.Delete.Request<TResource> {
      ...ParentKeyOf<TResource>;
    }

    model Response<TResource extends Model> is T.Resource.Delete.Response<TResource>;
  }

  namespace Reset {
    model Request<TResource extends Model> {
      ...ParentKeyOf<TResource>;
      ...KeyOf<TResource>;
    }

    model Response<TResource extends Model> {}
  }

  interface Interface<
    TResource extends Model,
    // List
    TListRequest extends Model,
    TListResponse extends Model,
    TListItem extends Model,
    // Create
    TCreateRequest extends Model,
    TCreateResponse extends Model,
    // Update
    TUpdateRequest extends Model,
    TUpdateResponse extends Model,
    // Delete
    TDeleteRequest extends Model,
    TDeleteResponse extends Model,
    // Reset
    TResetRequest extends Model,
    TResetResponse extends Model
  >
    extends T.Resource.List.Interface<
        TResource = TResource,
        TRequest = TListRequest,
        TItem = TListItem,
        TResponse = TListResponse
      >,
      T.Resource.Create.Interface<
        TResource = TResource,
        TRequest = TCreateRequest,
        TResponse = TCreateResponse,
        TListRequest = TListRequest,
        TListItem = TListItem
      >,
      T.Resource.Delete.Interface<TResource = TResource, TRequest = TDeleteRequest, TResponse = TDeleteResponse> {
    @rename("{name}Update", TResource)
    @DataClient.endpoint(
      "delta.js:deltaUpdate",
      {
        schema: DataClient.Schema<TResource>,
      }
    )
    Update is T.Resource.Update.Operation<
      TResource = TResource,
      TRequest = TUpdateRequest,
      TResponse = TUpdateResponse
    >;

    @rename("{name}Reset", TResource)
    @DataClient.endpoint(
      "delta.js:deltaReset",
      {
        schema: DataClient.Schema<TResource>,
      }
    )
    Reset(...TResetRequest): TResetResponse;
  }

  namespace _BOILERPLATE_ {
    model _RESOURCE_NAME_ {}

    model _RESOURCE_NAME_Delta is T.Delta.Item<_RESOURCE_NAME_>;

    model _RESOURCE_NAME_DeltaListRequest is T.Delta.List.Request<_RESOURCE_NAME_Delta>;
    model _RESOURCE_NAME_DeltaListResponse
      is T.Delta.List.Response<TRequest = _RESOURCE_NAME_DeltaListRequest, TItem = _RESOURCE_NAME_DeltaListItem>;
    model _RESOURCE_NAME_DeltaListItem is T.Delta.List.Item<_RESOURCE_NAME_Delta>;

    model _RESOURCE_NAME_DeltaCreateRequest is T.Delta.Create.Request<_RESOURCE_NAME_Delta>;
    model _RESOURCE_NAME_DeltaCreateResponse is T.Delta.Create.Response<_RESOURCE_NAME_Delta>;

    model _RESOURCE_NAME_DeltaUpdateRequest is T.Delta.Update.Request<_RESOURCE_NAME_Delta>;
    model _RESOURCE_NAME_DeltaUpdateResponse is T.Delta.Update.Response<_RESOURCE_NAME_Delta>;

    model _RESOURCE_NAME_DeltaDeleteRequest is T.Delta.Delete.Request<_RESOURCE_NAME_Delta>;
    model _RESOURCE_NAME_DeltaDeleteResponse is T.Delta.Delete.Response<_RESOURCE_NAME_Delta>;

    model _RESOURCE_NAME_DeltaResetRequest is T.Delta.Reset.Request<_RESOURCE_NAME_Delta>;
    model _RESOURCE_NAME_DeltaResetResponse is T.Delta.Reset.Response<_RESOURCE_NAME_Delta>;

    interface TDelta
      extends T.Delta.Interface<
          TResource = _RESOURCE_NAME_Delta,
          // List
          TListRequest = _RESOURCE_NAME_DeltaListRequest,
          TListResponse = _RESOURCE_NAME_DeltaListResponse,
          TListItem = _RESOURCE_NAME_DeltaListItem,
          // Create
          TCreateRequest = _RESOURCE_NAME_DeltaCreateRequest,
          TCreateResponse = _RESOURCE_NAME_DeltaCreateResponse,
          // Update
          TUpdateRequest = _RESOURCE_NAME_DeltaUpdateRequest,
          TUpdateResponse = _RESOURCE_NAME_DeltaUpdateResponse,
          // Delete
          TDeleteRequest = _RESOURCE_NAME_DeltaDeleteRequest,
          TDeleteResponse = _RESOURCE_NAME_DeltaDeleteResponse,
          // Reset
          TResetRequest = _RESOURCE_NAME_DeltaResetRequest,
          TResetResponse = _RESOURCE_NAME_DeltaResetResponse
        > {}
  }
}
