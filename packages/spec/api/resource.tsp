import "@typespec/protobuf";

import "../lib";

import "./change.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Reflection;
using TypeSpec.Rest.Resource;

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

namespace List {
  @normalize(Resource)
  @withVisibility(Query.List)
  model Item<Resource extends Model> {
    ...Resource;
  }

  model Request<Resource> {
    ...ParentKeyOf<Resource>;
  }

  @autoFields
  @normalize
  model Response<Resource, Item = Resource> {
    ...Request<Resource>;
    items: Item[];
  }
}

namespace Get {
  model Request<Resource> {
    ...KeyOf<Resource>;
  }

  @normalize(Resource)
  @withVisibility(Query.Get)
  model Response<Resource extends Model> {
    ...Resource;
  }
}

namespace Create {
  @normalize(Resource)
  @withVisibility(Mutation.Create)
  model Request<Resource extends Model> {
    ...ParentKeyOf<Resource>;
    ...Resource;
  }

  @autoChange({
    $data: {
      kind: API.Change.SourceKind.MERGE,
      $type: Resource,
    },
    $list: [
      {
        kind: API.Change.ListChangeKind.LIST_CHANGE_KIND_APPEND,
        $parent: {
          kind: API.Change.SourceKind.REQUEST,
          $type: ListResponse,
        },
      }
    ],
  })
  model Response<Resource extends Model, ListResponse extends Model = Resource> {
    ...KeyOf<Resource>;
    ...API.Change.Changes;
  }
}

namespace Update {
  @normalize(Resource)
  @withVisibility(Mutation.Update)
  model Request<Resource extends Model> {
    ...KeyOf<Resource>;
    ...OptionalProperties<OmitKey<Resource>>;
  }

  @autoChange({
    kind: API.Change.ChangeKind.CHANGE_KIND_UPDATE,
    $data: {
      kind: API.Change.SourceKind.REQUEST,
      $type: Resource,
    },
  })
  model Response<Resource extends Model> {
    ...API.Change.Changes;
  }
}

namespace Delete {
  @normalize(Resource)
  model Request<Resource extends Model> {
    ...KeyOf<Resource>;
  }

  @autoChange({
    kind: API.Change.ChangeKind.CHANGE_KIND_DELETE,
    $data: {
      kind: API.Change.SourceKind.REQUEST,
      $type: Resource,
    },
  })
  model Response<Resource extends Model> {
    ...API.Change.Changes;
  }
}
