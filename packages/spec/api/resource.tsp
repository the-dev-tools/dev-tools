import "@typespec/protobuf";

import "./lib.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Reflection;

@package({
  name: "resources.v1",
  options: {
    go_package: "the-dev-tools/spec/dist/buf/go/resources/v1;resourcesv1",
  },
})
namespace API.Resource;

alias Id = bytes;

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

enum MovePosition {
  MOVE_POSITION_UNSPECIFIED: 0,
  MOVE_POSITION_AFTER: 1,
  MOVE_POSITION_BEFORE: 2,
}

interface CRUD<
  TResource extends Model,
  // List
  TListRequest extends Model = List.Request<TResource>,
  TListItem extends Model = List.Item<TResource>,
  TListResponse extends Model = List.Response<TResource, TRequest = TListRequest, TItem = TListItem>,
  // Get
  TGetRequest extends Model = Get.Request<TResource>,
  TGetResponse extends Model = Get.Response<TResource>,
  // Create
  TCreateRequest extends Model = Create.Request<TResource>,
  TCreateResponse extends Model = Create.Response<TResource, TListResponse = TListResponse>,
  // Update
  TUpdateRequest extends Model = Update.Request<TResource>,
  TUpdateResponse extends Model = Update.Response<TResource>,
  // Delete
  TDeleteRequest extends Model = Delete.Request<TResource>,
  TDeleteResponse extends Model = Delete.Response<TResource>
>
  extends List.Interface<TResource, TRequest = TListRequest, TResponse = TListResponse, TItem = TListItem>,
    Get.Interface<TResource, TRequest = TGetRequest, TResponse = TGetResponse>,
    Create.Interface<
      TResource,
      TRequest = TCreateRequest,
      TResponse = TCreateResponse,
      TListRequest = TListRequest,
      TListItem = TListItem
    >,
    Update.Interface<TResource, TRequest = TUpdateRequest, TResponse = TUpdateResponse>,
    Delete.Interface<TResource, TRequest = TDeleteRequest, TResponse = TDeleteResponse> {}

namespace List {
  interface Interface<
    TResource extends Model,
    TMove extends Model = TResource,
    TRequest extends Model = Request<TResource, TMove = TMove>,
    TItem extends Model = Item<TResource, TMove = TMove>,
    TResponse extends Model = Response<TResource, TRequest = TRequest, TItem = TItem, TMove = TMove>
  > {
    @useFriendlyName
    @friendlyName("{name}List", TResource)
    List is Operation<TResource, TRequest = TRequest, TResponse = TResponse, TItem = TItem, TMove = TMove>;
  }

  @move(List, TMove)
  @endpoint(
    "resource.js:list",
    {
      inputPrimaryKeys: PrimaryKeys<TRequest>,
      itemSchema: Schema<TItem>,
    }
  )
  op Operation<
    TResource extends Model,
    TMove extends Model = TResource,
    TRequest extends Model = Request<TResource, TMove = TMove>,
    TItem extends Model = Item<TResource, TMove = TMove>,
    TResponse extends Model = Response<TResource, TRequest = TRequest, TItem = TItem, TMove = TMove>
  >(...TRequest): TResponse;

  @move(List, TMove)
  @friendlyName("{name}ListItem", TResource)
  @entity(TResource)
  @withVisibility(Query.List)
  @autoFields
  model Item<TResource extends Model, TMove extends Model = TResource> {
    ...TResource;
  }

  @move(List, TMove)
  @friendlyName("{name}ListRequest", TResource)
  model Request<TResource extends Model, TMove extends Model = TResource> {
    ...ParentKeyOf<TResource>;
  }

  @move(List, TMove)
  @friendlyName("{name}ListResponse", TResource)
  @autoFields
  model Response<
    TResource extends Model,
    TMove extends Model = TResource,
    TRequest extends Model = Request<TResource, TMove = TMove>,
    TItem extends Model = List.Item<TResource, TMove = TMove>
  > {
    ...TRequest;
    items: TItem[];
  }
}

namespace Get {
  interface Interface<
    TResource extends Model,
    TMove extends Model = TResource,
    TRequest extends Model = Request<TResource, TMove = TMove>,
    TResponse extends Model = Response<TResource, TMove = TMove>
  > {
    @useFriendlyName
    @friendlyName("{name}Get", TResource)
    Get is Operation<TResource, TRequest = TRequest, TResponse = TResponse, TMove = TMove>;
  }

  @move(Get, TMove)
  @endpoint(
    "resource.js:get",
    {
      schema: Schema<TResponse>,
    }
  )
  op Operation<
    TResource extends Model,
    TMove extends Model = TResource,
    TRequest extends Model = Request<TResource, TMove = TMove>,
    TResponse extends Model = Response<TResource, TMove = TMove>
  >(...TRequest): TResponse;

  @move(Get, TMove)
  @friendlyName("{name}GetRequest", TResource)
  model Request<TResource extends Model, TMove extends Model = TResource> {
    ...KeyOf<TResource>;
  }

  @move(Get, TMove)
  @friendlyName("{name}GetResponse", TResource)
  @entity(TResource)
  @withVisibility(Query.Get)
  model Response<TResource extends Model, TMove extends Model = TResource> {
    ...TResource;
  }
}

namespace Create {
  interface Interface<
    TResource extends Model,
    TMove extends Model = TResource,
    TRequest extends Model = Request<TResource, TMove = TMove>,
    TResponse extends Model = Response<TResource, TMove = TMove>,
    TListRequest extends Model = List.Request<TResource, TMove = TMove>,
    TListItem extends Model = List.Item<TResource, TMove = TMove>
  > {
    @useFriendlyName
    @friendlyName("{name}Create", TResource)
    Create is Operation<
      TResource,
      TRequest = TRequest,
      TResponse = TResponse,
      TListRequest = TListRequest,
      TListItem = TListItem,
      TMove = TMove
    >;
  }

  @move(Create, TMove)
  @endpoint(
    "resource.js:create",
    {
      listInputPrimaryKeys: PrimaryKeys<TListRequest>,
      listItemSchema: Schema<TListItem>,
    }
  )
  op Operation<
    TResource extends Model,
    TMove extends Model = TResource,
    TRequest extends Model = Request<TResource, TMove = TMove>,
    TResponse extends Model = Response<TResource, TMove = TMove>,
    TListRequest extends Model = List.Request<TResource, TMove = TMove>,
    TListItem extends Model = List.Item<TResource, TMove = TMove>
  >(...TRequest): TResponse;

  @move(Create, TMove)
  @friendlyName("{name}CreateRequest", TResource)
  @withVisibility(Mutation.Create)
  model Request<TResource extends Model, TMove extends Model = TResource> {
    ...ParentKeyOf<TResource>;
    ...TResource;
  }

  @move(Create, TMove)
  @friendlyName("{name}CreateResponse", TResource)
  model Response<TResource extends Model, TMove extends Model = TResource, TListResponse extends Model = TResource> {
    ...KeyOf<TResource>;
  }
}

namespace Update {
  interface Interface<
    TResource extends Model,
    TMove extends Model = TResource,
    TRequest extends Model = Request<TResource, TMove = TMove>,
    TResponse extends Model = Response<TResource, TMove = TMove>
  > {
    @useFriendlyName
    @friendlyName("{name}Update", TResource)
    Update is Operation<TResource, TRequest = TRequest, TResponse = TResponse, TMove = TMove>;
  }

  @move(Update, TMove)
  @endpoint(
    "resource.js:update",
    {
      schema: Schema<TResource>,
    }
  )
  op Operation<
    TResource extends Model,
    TMove extends Model = TResource,
    TRequest extends Model = Request<TResource, TMove = TMove>,
    TResponse extends Model = Response<TResource, TMove = TMove>
  >(...TRequest): TResponse;

  @move(Update, TMove)
  @friendlyName("{name}UpdateRequest", TResource)
  @withVisibility(Mutation.Update)
  model Request<TResource extends Model, TMove extends Model = TResource> {
    ...KeyOf<TResource>;
    ...OptionalProperties<OmitKey<TResource>>;
  }

  @move(Update, TMove)
  @friendlyName("{name}UpdateResponse", TResource)
  model Response<TResource extends Model, TMove extends Model = TResource> {}
}

namespace Delete {
  interface Interface<
    TResource extends Model,
    TMove extends Model = TResource,
    TRequest extends Model = Request<TResource, TMove = TMove>,
    TResponse extends Model = Response<TResource, TMove = TMove>
  > {
    @useFriendlyName
    @friendlyName("{name}Delete", TResource)
    Delete is Operation<TResource, TRequest = TRequest, TResponse = TResponse, TMove = TMove>;
  }

  @move(Delete, TMove)
  @endpoint(
    "resource.js:delete$",
    {
      schema: Schema<TResource>,
    }
  )
  op Operation<
    TResource extends Model,
    TMove extends Model = TResource,
    TRequest extends Model = Request<TResource, TMove = TMove>,
    TResponse extends Model = Response<TResource, TMove = TMove>
  >(...TRequest): TResponse;

  @move(Delete, TMove)
  @friendlyName("{name}DeleteRequest", TResource)
  model Request<TResource extends Model, TMove extends Model = TResource> {
    ...KeyOf<TResource>;
  }

  @move(Delete, TMove)
  @friendlyName("{name}DeleteResponse", TResource)
  model Response<TResource extends Model, TMove extends Model = TResource> {}
}
