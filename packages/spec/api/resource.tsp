using DevTools;
using TypeSpec.Reflection;

namespace API.Resource {
  alias Id = bytes;

  enum MovePosition {
    After,
    Before,
  }
}

namespace T.Resource {
  // TODO
  // replace `None` visibilities with `@invisible` and `withVisibility` with
  // `withVisibilityFilter`after upstream bug is fixed
  // https://github.com/microsoft/typespec/issues/6280

  @defaultVisibility(Query.Get, Query.List)
  enum Query {
    None,
    Get,
    List,
  }

  @defaultVisibility(Mutation.Create, Mutation.Update)
  enum Mutation {
    None,
    Create,
    Update,
  }

  interface CRUD<
    TResource extends Model,
    // List
    TListRequest extends Model,
    TListResponse extends Model,
    TListItem extends Model,
    // Get
    TGetRequest extends Model,
    TGetResponse extends Model,
    // Create
    TCreateRequest extends Model,
    TCreateResponse extends Model,
    // Update
    TUpdateRequest extends Model,
    TUpdateResponse extends Model,
    // Delete
    TDeleteRequest extends Model,
    TDeleteResponse extends Model
  >
    extends List.Interface<
        TResource = TResource,
        TRequest = TListRequest,
        TResponse = TListResponse,
        TItem = TListItem
      >,
      Get.Interface<TResource = TResource, TRequest = TGetRequest, TResponse = TGetResponse>,
      Create.Interface<
        TResource = TResource,
        TRequest = TCreateRequest,
        TResponse = TCreateResponse,
        TListRequest = TListRequest,
        TListItem = TListItem
      >,
      Update.Interface<TResource = TResource, TRequest = TUpdateRequest, TResponse = TUpdateResponse>,
      Delete.Interface<TResource = TResource, TRequest = TDeleteRequest, TResponse = TDeleteResponse> {}

  namespace List {
    interface Interface<TResource extends Model, TRequest extends Model, TResponse extends Model, TItem extends Model> {
      @rename("{name}List", TResource)
      List is Operation<TRequest = TRequest, TResponse = TResponse, TItem = TItem>;
    }

    @DataClient.endpoint(
      "resource.js:list",
      {
        inputPrimaryKeys: DataClient.PrimaryKeys<TRequest>,
        itemSchema: DataClient.Schema<TItem>,
      }
    )
    op Operation<TRequest extends Model, TResponse extends Model, TItem extends Model>(...TRequest): TResponse;

    model Request<TResource extends Model> {
      ...ParentKeyOf<TResource>;
    }

    model Response<TRequest extends Model, TItem extends Model> {
      ...TRequest;
      items: TItem[];
    }

    @DataClient.entity(TResource)
    @withVisibility(Query.List)
    model Item<TResource extends Model> {
      ...TResource;
    }
  }

  namespace Get {
    interface Interface<TResource extends Model, TRequest extends Model, TResponse extends Model> {
      @rename("{name}Get", TResource)
      Get is Operation<TRequest = TRequest, TResponse = TResponse>;
    }

    @DataClient.endpoint(
      "resource.js:get",
      {
        schema: DataClient.Schema<TResponse>,
      }
    )
    op Operation<TRequest extends Model, TResponse extends Model>(...TRequest): TResponse;

    model Request<TResource extends Model> {
      ...KeyOf<TResource>;
    }

    @DataClient.entity(TResource)
    @withVisibility(Query.Get)
    model Response<TResource extends Model> {
      ...TResource;
    }
  }

  namespace Create {
    interface Interface<
      TResource extends Model,
      TRequest extends Model,
      TResponse extends Model,
      TListRequest extends Model,
      TListItem extends Model
    > {
      @rename("{name}Create", TResource)
      Create is Operation<
        TRequest = TRequest,
        TResponse = TResponse,
        TListRequest = TListRequest,
        TListItem = TListItem
      >;
    }

    @DataClient.endpoint(
      "resource.js:create",
      {
        listInputPrimaryKeys: DataClient.PrimaryKeys<TListRequest>,
        listItemSchema: DataClient.Schema<TListItem>,
      }
    )
    op Operation<TRequest extends Model, TResponse extends Model, TListRequest extends Model, TListItem extends Model>(
      ...TRequest,
    ): TResponse;

    @withVisibility(Mutation.Create)
    model Request<TResource extends Model> {
      ...ParentKeyOf<TResource>;
      ...TResource;
    }

    model Response<TResource extends Model> {
      ...KeyOf<TResource>;
    }
  }

  namespace Update {
    interface Interface<TResource extends Model, TRequest extends Model, TResponse extends Model> {
      @rename("{name}Update", TResource)
      Update is Operation<TResource = TResource, TRequest = TRequest, TResponse = TResponse>;
    }

    @DataClient.endpoint(
      "resource.js:update",
      {
        schema: DataClient.Schema<TResource>,
      }
    )
    op Operation<TResource extends Model, TRequest extends Model, TResponse extends Model>(...TRequest): TResponse;

    @withVisibility(Mutation.Update)
    model Request<TResource extends Model> {
      ...KeyOf<TResource>;
      ...OptionalProperties<OmitKey<TResource>>;
    }

    model Response<TResource extends Model> {}
  }

  namespace Delete {
    interface Interface<TResource extends Model, TRequest extends Model, TResponse extends Model> {
      @rename("{name}Delete", TResource)
      Delete is Operation<TResource = TResource, TRequest = TRequest, TResponse = TResponse>;
    }

    @DataClient.endpoint(
      "resource.js:delete$",
      {
        schema: DataClient.Schema<TResource>,
      }
    )
    op Operation<TResource extends Model, TRequest extends Model, TResponse extends Model>(...TRequest): TResponse;

    model Request<TResource extends Model> {
      ...KeyOf<TResource>;
    }

    model Response<TResource extends Model> {}
  }

  namespace _BOILERPLATE_ {
    model _RESOURCE_NAME_ {}

    model _RESOURCE_NAME_ListRequest is T.Resource.List.Request<_RESOURCE_NAME_>;
    model _RESOURCE_NAME_ListResponse
      is T.Resource.List.Response<TRequest = _RESOURCE_NAME_ListRequest, TItem = _RESOURCE_NAME_ListItem>;
    model _RESOURCE_NAME_ListItem is T.Resource.List.Item<_RESOURCE_NAME_>;

    model _RESOURCE_NAME_GetRequest is T.Resource.Get.Request<_RESOURCE_NAME_>;
    model _RESOURCE_NAME_GetResponse is T.Resource.Get.Response<_RESOURCE_NAME_>;

    model _RESOURCE_NAME_CreateRequest is T.Resource.Create.Request<_RESOURCE_NAME_>;
    model _RESOURCE_NAME_CreateResponse is T.Resource.Create.Response<_RESOURCE_NAME_>;

    model _RESOURCE_NAME_UpdateRequest is T.Resource.Update.Request<_RESOURCE_NAME_>;
    model _RESOURCE_NAME_UpdateResponse is T.Resource.Update.Response<_RESOURCE_NAME_>;

    model _RESOURCE_NAME_DeleteRequest is T.Resource.Delete.Request<_RESOURCE_NAME_>;
    model _RESOURCE_NAME_DeleteResponse is T.Resource.Delete.Response<_RESOURCE_NAME_>;

    interface TCRUD
      extends T.Resource.CRUD<
          TResource = _RESOURCE_NAME_,
          // List
          TListRequest = _RESOURCE_NAME_ListRequest,
          TListResponse = _RESOURCE_NAME_ListResponse,
          TListItem = _RESOURCE_NAME_ListItem,
          // Get
          TGetRequest = _RESOURCE_NAME_GetRequest,
          TGetResponse = _RESOURCE_NAME_GetResponse,
          // Create
          TCreateRequest = _RESOURCE_NAME_CreateRequest,
          TCreateResponse = _RESOURCE_NAME_CreateResponse,
          // Update
          TUpdateRequest = _RESOURCE_NAME_UpdateRequest,
          TUpdateResponse = _RESOURCE_NAME_UpdateResponse,
          // Delete
          TDeleteRequest = _RESOURCE_NAME_DeleteRequest,
          TDeleteResponse = _RESOURCE_NAME_DeleteResponse
        > {}

    interface TList
      extends T.Resource.List.Interface<
          TResource = _RESOURCE_NAME_,
          TRequest = _RESOURCE_NAME_ListRequest,
          TResponse = _RESOURCE_NAME_ListResponse,
          TItem = _RESOURCE_NAME_ListItem
        > {}

    interface TGet
      extends T.Resource.Get.Interface<
          TResource = _RESOURCE_NAME_,
          TRequest = _RESOURCE_NAME_GetRequest,
          TResponse = _RESOURCE_NAME_GetResponse
        > {}

    interface TCreate
      extends T.Resource.Create.Interface<
          TResource = _RESOURCE_NAME_,
          TRequest = _RESOURCE_NAME_CreateRequest,
          TResponse = _RESOURCE_NAME_CreateResponse,
          TListRequest = _RESOURCE_NAME_ListRequest,
          TListItem = _RESOURCE_NAME_ListItem
        > {}

    interface TUpdate
      extends T.Resource.Update.Interface<
          TResource = _RESOURCE_NAME_,
          TRequest = _RESOURCE_NAME_UpdateRequest,
          TResponse = _RESOURCE_NAME_UpdateResponse
        > {}

    interface TDelete
      extends T.Resource.Delete.Interface<
          TResource = _RESOURCE_NAME_,
          TRequest = _RESOURCE_NAME_DeleteRequest,
          TResponse = _RESOURCE_NAME_DeleteResponse
        > {}
  }
}
