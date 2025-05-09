import "@typespec/protobuf";

import "./lib.tsp";

using TypeSpec.Reflection;

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

interface CRUD<Resource extends Model>
  extends List.Operation<Resource>,
    Get.Operation<Resource>,
    Create.Operation<Resource>,
    Update.Operation<Resource>,
    Delete.Operation<Resource> {}

namespace List {
  interface Operation<Resource extends Model> {
    @endpoint
    @useFriendlyName
    @friendlyName("{name}List", Resource)
    List(...Request<Resource>): Response<Resource>;
  }

  @move(List, Resource)
  @friendlyName("{name}ListItem", Resource)
  @entity(Resource)
  @normalize(Resource)
  @withVisibility(Query.List)
  model Item<Resource extends Model> {
    ...Resource;
  }

  @move(List, Resource)
  @friendlyName("{name}ListRequest", Resource)
  model Request<Resource extends Model> {
    ...ParentKeyOf<Resource>;
  }

  @move(List, Resource)
  @friendlyName("{name}ListResponse", Resource)
  @autoFields
  @normalize
  model Response<Resource extends Model, Item = List.Item<Resource>> {
    ...Request<Resource>;
    // @collection
    // @argKeys(Request<Resource>)
    items: Item[];
  }
}

namespace Get {
  interface Operation<Resource extends Model> {
    @endpoint
    @useFriendlyName
    @friendlyName("{name}Get", Resource)
    Get(...Request<Resource>): Response<Resource>;
  }

  @move(Get, Resource)
  @friendlyName("{name}GetRequest", Resource)
  model Request<Resource extends Model> {
    ...KeyOf<Resource>;
  }

  @move(Get, Resource)
  @friendlyName("{name}GetResponse", Resource)
  @normalize(Resource)
  @entity(Resource)
  @withVisibility(Query.Get)
  model Response<Resource extends Model> {
    ...Resource;
  }
}

namespace Create {
  interface Operation<Resource extends Model> {
    @endpoint
    @useFriendlyName
    @friendlyName("{name}Create", Resource)
    Create(...Request<Resource>): Response<Resource>;
  }

  @move(Create, Resource)
  @friendlyName("{name}CreateRequest", Resource)
  // @normalize(Resource)
  @withVisibility(Mutation.Create)
  model Request<Resource extends Model> {
    ...ParentKeyOf<Resource>;
    ...Resource;
  }

  @move(Create, Resource)
  @friendlyName("{name}CreateResponse", Resource)
  // @collectionPush(Request<Resource>)
  model Response<Resource extends Model, ListResponse extends Model = Resource> {
    ...KeyOf<Resource>;
  }
}

namespace Update {
  interface Operation<Resource extends Model> {
    @endpoint
    @useFriendlyName
    @friendlyName("{name}Update", Resource)
    Update(...Request<Resource>): Response<Resource>;
  }

  @move(Update, Resource)
  @friendlyName("{name}UpdateRequest", Resource)
  // @normalize(Resource)
  @withVisibility(Mutation.Update)
  model Request<Resource extends Model> {
    ...KeyOf<Resource>;
    ...OptionalProperties<OmitKey<Resource>>;
  }

  @move(Update, Resource)
  @friendlyName("{name}UpdateResponse", Resource)
  model Response<Resource extends Model> {}
}

namespace Delete {
  interface Operation<Resource extends Model> {
    @endpoint
    @useFriendlyName
    @friendlyName("{name}Delete", Resource)
    Delete(...Request<Resource>): Response<Resource>;
  }

  @move(Delete, Resource)
  @friendlyName("{name}DeleteRequest", Resource)
  // @normalize(Resource)
  model Request<Resource extends Model> {
    ...KeyOf<Resource>;
  }

  @move(Delete, Resource)
  @friendlyName("{name}DeleteResponse", Resource)
  model Response<Resource extends Model> {}
}
