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
    TRequest extends Model = Request<TResource>,
    TItem extends Model = Item<TResource>,
    TResponse extends Model = Response<TResource, TRequest = TRequest, TItem = TItem>
  > {
    @useFriendlyName
    @friendlyName("{name}List", TResource)
    List is Operation<TResource, TRequest = TRequest, TResponse = TResponse, TItem = TItem>;
  }

  @move(List, TResource)
  @endpoint(
    "resource.js:list",
    {
      inputPrimaryKeys: PrimaryKeys<TRequest>,
      itemSchema: Schema<TItem>,
    }
  )
  op Operation<
    TResource extends Model,
    TRequest extends Model = Request<TResource>,
    TItem extends Model = Item<TResource>,
    TResponse extends Model = Response<TResource, TRequest = TRequest, TItem = TItem>
  >(...TRequest): TResponse;

  @move(List, TResource)
  @friendlyName("{name}ListItem", TResource)
  @entity(TResource)
  @withVisibility(Query.List)
  model Item<TResource extends Model> {
    ...TResource;
  }

  @move(List, TResource)
  @friendlyName("{name}ListRequest", TResource)
  model Request<TResource extends Model> {
    ...ParentKeyOf<TResource>;
  }

  @move(List, TResource)
  @friendlyName("{name}ListResponse", TResource)
  @autoFields
  model Response<
    TResource extends Model,
    TRequest extends Model = Request<TResource>,
    TItem extends Model = List.Item<TResource>
  > {
    ...TRequest;
    items: TItem[];
  }
}

namespace Get {
  interface Interface<
    TResource extends Model,
    TRequest extends Model = Request<TResource>,
    TResponse extends Model = Response<TResource>
  > {
    @useFriendlyName
    @friendlyName("{name}Get", TResource)
    Get is Operation<TResource, TRequest = TRequest, TResponse = TResponse>;
  }

  @move(Get, TResource)
  @endpoint(
    "resource.js:get",
    {
      schema: Schema<TResponse>,
    }
  )
  op Operation<
    TResource extends Model,
    TRequest extends Model = Request<TResource>,
    TResponse extends Model = Response<TResource>
  >(...TRequest): TResponse;

  @move(Get, TResource)
  @friendlyName("{name}GetRequest", TResource)
  model Request<TResource extends Model> {
    ...KeyOf<TResource>;
  }

  @move(Get, TResource)
  @friendlyName("{name}GetResponse", TResource)
  @entity(TResource)
  @withVisibility(Query.Get)
  model Response<TResource extends Model> {
    ...TResource;
  }
}

namespace Create {
  interface Interface<
    TResource extends Model,
    TRequest extends Model = Request<TResource>,
    TResponse extends Model = Response<TResource>,
    TListRequest extends Model = List.Request<TResource>,
    TListItem extends Model = List.Item<TResource>
  > {
    @useFriendlyName
    @friendlyName("{name}Create", TResource)
    Create is Operation<
      TResource,
      TRequest = TRequest,
      TResponse = TResponse,
      TListRequest = TListRequest,
      TListItem = TListItem
    >;
  }

  @move(Create, TResource)
  @endpoint(
    "resource.js:create",
    {
      listInputPrimaryKeys: PrimaryKeys<TListRequest>,
      listItemSchema: Schema<TListItem>,
    }
  )
  op Operation<
    TResource extends Model,
    TRequest extends Model = Request<TResource>,
    TResponse extends Model = Response<TResource>,
    TListRequest extends Model = List.Request<TResource>,
    TListItem extends Model = List.Item<TResource>
  >(...TRequest): TResponse;

  @move(Create, TResource)
  @friendlyName("{name}CreateRequest", TResource)
  @withVisibility(Mutation.Create)
  model Request<TResource extends Model> {
    ...ParentKeyOf<TResource>;
    ...TResource;
  }

  @move(Create, TResource)
  @friendlyName("{name}CreateResponse", TResource)
  model Response<TResource extends Model, TListResponse extends Model = TResource> {
    ...KeyOf<TResource>;
  }
}

namespace Update {
  interface Interface<
    TResource extends Model,
    TRequest extends Model = Request<TResource>,
    TResponse extends Model = Response<TResource>
  > {
    @useFriendlyName
    @friendlyName("{name}Update", TResource)
    Update is Operation<TResource, TRequest = TRequest, TResponse = TResponse>;
  }

  @move(Update, TResource)
  @endpoint(
    "resource.js:update",
    {
      schema: Schema<TResource>,
    }
  )
  op Operation<
    TResource extends Model,
    TRequest extends Model = Request<TResource>,
    TResponse extends Model = Response<TResource>
  >(...TRequest): TResponse;

  @move(Update, TResource)
  @friendlyName("{name}UpdateRequest", TResource)
  @withVisibility(Mutation.Update)
  model Request<TResource extends Model> {
    ...KeyOf<TResource>;
    ...OptionalProperties<OmitKey<TResource>>;
  }

  @move(Update, TResource)
  @friendlyName("{name}UpdateResponse", TResource)
  model Response<TResource extends Model> {}
}

namespace Delete {
  interface Interface<
    TResource extends Model,
    TRequest extends Model = Request<TResource>,
    TResponse extends Model = Response<TResource>
  > {
    @useFriendlyName
    @friendlyName("{name}Delete", TResource)
    Delete is Operation<TResource, TRequest = TRequest, TResponse = TResponse>;
  }

  @move(Delete, TResource)
  @endpoint(
    "resource.js:delete$",
    {
      schema: Schema<TResource>,
    }
  )
  op Operation<
    TResource extends Model,
    TRequest extends Model = Request<TResource>,
    TResponse extends Model = Response<TResource>
  >(...TRequest): TResponse;

  @move(Delete, TResource)
  @friendlyName("{name}DeleteRequest", TResource)
  model Request<TResource extends Model> {
    ...KeyOf<TResource>;
  }

  @move(Delete, TResource)
  @friendlyName("{name}DeleteResponse", TResource)
  model Response<TResource extends Model> {}
}
