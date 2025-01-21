import "@typespec/protobuf";

import "../lib";

import "./change.tsp";

using TypeSpec.Protobuf;
using TypeSpec.Reflection;
using TypeSpec.Rest.Resource;

namespace API.Resource;

alias Id = bytes;

@normalize(Resource)
model Change<Resource extends Model> {
  ...KeyOf<Resource>;
  ...OptionalProperties<OmitKey<Resource>>;
}

namespace List {
  @normalize(Resource)
  @withDefaultKeyVisibility("query")
  @withVisibility("query", "list")
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
  @withDefaultKeyVisibility("query")
  @withVisibility("query", "get")
  model Response<Resource extends Model> {
    ...Resource;
  }
}

namespace Create {
  @normalize(Resource)
  @withDefaultKeyVisibility("mutate")
  @withVisibility("mutate", "create")
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
  @normalize(Resource)
  model Response<Resource extends Model, ListResponse extends Model = Resource> {
    ...KeyOf<Resource>;
    ...API.Change.Changes;
  }
}

namespace Update {
  @normalize(Resource)
  @withDefaultKeyVisibility("mutate")
  @withVisibility("mutate", "update")
  model Request<Resource extends Model> {
    ...KeyOf<Resource>;
    ...ParentKeyOf<Resource>;
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
    ...ParentKeyOf<Resource>;
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
