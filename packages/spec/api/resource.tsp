import "@typespec/protobuf";

import "./lib.tsp";

import "./change.tsp";

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

@normalize(Resource)
model Change<Resource extends Model> {
  ...KeyOf<Resource>;
  ...OptionalProperties<OmitKey<Resource>>;
}

interface CRUD<TResource extends Model>
  extends List.Interface<TResource>,
    Get.Interface<TResource>,
    Create.Interface<TResource>,
    Update.Interface<TResource>,
    Delete.Interface<TResource> {}

namespace List {
  interface Interface<TResource extends Model> {
    @useFriendlyName
    @friendlyName("{name}List", TResource)
    List is Operation<TResource>;
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
    TResponse extends Model = Response<TResource>,
    TItem extends Model = Item<TResource>
  >(...TRequest): TResponse;

  @move(List, TResource)
  @friendlyName("{name}ListItem", TResource)
  @entity(TResource)
  @normalize(TResource)
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
  @normalize
  model Response<TResource extends Model, TItem = List.Item<TResource>> {
    ...Request<TResource>;
    items: TItem[];
  }
}

namespace Get {
  interface Interface<TResource extends Model> {
    @useFriendlyName
    @friendlyName("{name}Get", TResource)
    Get is Operation<TResource>;
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
  @normalize(TResource)
  @entity(TResource)
  @withVisibility(Query.Get)
  model Response<TResource extends Model> {
    ...TResource;
  }
}

namespace Create {
  interface Interface<TResource extends Model> {
    @useFriendlyName
    @friendlyName("{name}Create", TResource)
    Create is Operation<TResource>;
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
  @normalize(TResource)
  @withVisibility(Mutation.Create)
  model Request<TResource extends Model> {
    ...ParentKeyOf<TResource>;
    ...TResource;
  }

  @move(Create, TResource)
  @friendlyName("{name}CreateResponse", TResource)
  @autoChange({
    $data: {
      kind: API.Change.SourceKind.MERGE,
      $type: TResource,
    },
    $list: [
      {
        kind: API.Change.ListChangeKind.LIST_CHANGE_KIND_APPEND,
        $parent: {
          kind: API.Change.SourceKind.REQUEST,
          $type: TListResponse,
        },
      }
    ],
  })
  model Response<TResource extends Model, TListResponse extends Model = TResource> {
    ...KeyOf<TResource>;
    ...API.Change.Changes;
  }
}

namespace Update {
  interface Interface<TResource extends Model> {
    @useFriendlyName
    @friendlyName("{name}Update", TResource)
    Update is Operation<TResource>;
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
  @normalize(TResource)
  @withVisibility(Mutation.Update)
  model Request<TResource extends Model> {
    ...KeyOf<TResource>;
    ...OptionalProperties<OmitKey<TResource>>;
  }

  @move(Update, TResource)
  @friendlyName("{name}UpdateResponse", TResource)
  @autoChange({
    kind: API.Change.ChangeKind.CHANGE_KIND_UPDATE,
    $data: {
      kind: API.Change.SourceKind.REQUEST,
      $type: TResource,
    },
  })
  model Response<TResource extends Model> {
    ...API.Change.Changes;
  }
}

namespace Delete {
  interface Interface<TResource extends Model> {
    @useFriendlyName
    @friendlyName("{name}Delete", TResource)
    Delete is Operation<TResource>;
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
  @normalize(TResource)
  model Request<TResource extends Model> {
    ...KeyOf<TResource>;
  }

  @move(Delete, TResource)
  @friendlyName("{name}DeleteResponse", TResource)
  @autoChange({
    kind: API.Change.ChangeKind.CHANGE_KIND_DELETE,
    $data: {
      kind: API.Change.SourceKind.REQUEST,
      $type: TResource,
    },
  })
  model Response<TResource extends Model> {
    ...API.Change.Changes;
  }
}
