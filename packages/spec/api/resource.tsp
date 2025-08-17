using DevTools;
using Reflection;

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
  After,
  Before,
}

@templateOf(TResource)
interface CRUD<TResource extends Model>
  extends List.Interface<TResource>,
    Get.Interface<TResource>,
    Create.Interface<TResource>,
    Update.Interface<TResource>,
    Delete.Interface<TResource> {}

namespace List {
  @templateOf(TResource)
  interface Interface<TResource extends Model> {
    List is Operation<TResource>;
  }

  @DataClient.endpoint(
    "resource.js:list",
    {
      inputPrimaryKeys: DataClient.PrimaryKeys<ListRequest<TResource>>,
      itemSchema: DataClient.Schema<ListItem<TResource>>,
    }
  )
  op Operation<TResource extends Model, TRequest = ListRequest<TResource>, TResponse = ListResponse<TResource>>(
    ...TRequest,
  ): TResponse;

  @DataClient.entity(TResource)
  @withVisibility(Query.List)
  @templateOf(TResource)
  model ListItem<TResource extends Model> {
    ...TResource;
  }

  @templateOf(TResource)
  model ListRequest<TResource extends Model> {
    ...ParentKeyOf<TResource>;
  }

  @templateOf(TResource)
  model ListResponse<TResource extends Model> {
    ...ListRequest<TResource>;
    items: ListItem<TResource>[];
  }
}

namespace Get {
  @templateOf(TResource)
  interface Interface<TResource extends Model> {
    Get is Operation<TResource>;
  }

  @DataClient.endpoint(
    "resource.js:get",
    {
      schema: DataClient.Schema<GetResponse<TResource>>,
    }
  )
  op Operation<TResource extends Model, TRequest = GetRequest<TResource>, TResponse = GetResponse<TResource>>(
    ...TRequest,
  ): TResponse;

  @templateOf(TResource)
  model GetRequest<TResource extends Model> {
    ...KeyOf<TResource>;
  }

  @DataClient.entity(TResource)
  @withVisibility(Query.Get)
  @templateOf(TResource)
  model GetResponse<TResource extends Model> {
    ...TResource;
  }
}

namespace Create {
  @templateOf(TResource)
  interface Interface<TResource extends Model> {
    Create is Operation<TResource>;
  }

  @DataClient.endpoint(
    "resource.js:create",
    {
      listInputPrimaryKeys: DataClient.PrimaryKeys<List.ListRequest<TResource>>,
      listItemSchema: DataClient.Schema<List.ListItem<TResource>>,
    }
  )
  op Operation<TResource extends Model, TRequest = CreateRequest<TResource>, TResponse = CreateResponse<TResource>>(
    ...TRequest,
  ): TResponse;

  @withVisibility(Mutation.Create)
  @templateOf(TResource)
  model CreateRequest<TResource extends Model> {
    ...ParentKeyOf<TResource>;
    ...TResource;
  }

  @templateOf(TResource)
  model CreateResponse<TResource extends Model> {
    ...KeyOf<TResource>;
  }
}

namespace Update {
  @templateOf(TResource)
  interface Interface<TResource extends Model> {
    Update is Operation<TResource>;
  }

  @DataClient.endpoint(
    "resource.js:update",
    {
      schema: DataClient.Schema<TResource>,
    }
  )
  op Operation<TResource extends Model, TRequest = UpdateRequest<TResource>, TResponse = UpdateResponse<TResource>>(
    ...TRequest,
  ): TResponse;

  @withVisibility(Mutation.Update)
  @templateOf(TResource)
  model UpdateRequest<TResource extends Model> {
    ...KeyOf<TResource>;
    ...OptionalProperties<OmitKey<TResource>>;
  }

  @templateOf(TResource)
  model UpdateResponse<TResource extends Model> {}
}

namespace Delete {
  @templateOf(TResource)
  interface Interface<TResource extends Model> {
    Delete is Operation<TResource>;
  }

  @DataClient.endpoint(
    "resource.js:delete$",
    {
      schema: DataClient.Schema<TResource>,
    }
  )
  op Operation<TResource extends Model, TRequest = DeleteRequest<TResource>, TResponse = DeleteResponse<TResource>>(
    ...TRequest,
  ): TResponse;

  @templateOf(TResource)
  model DeleteRequest<TResource extends Model> {
    ...KeyOf<TResource>;
  }

  @templateOf(TResource)
  model DeleteResponse<TResource extends Model> {}
}
