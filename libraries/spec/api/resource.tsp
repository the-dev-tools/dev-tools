import "@typespec/protobuf";

import "../lib";

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

model Changes {
  @autoField changes?: WellKnown.Any[];
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
  model Response<Item> {
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
  @withDefaultKeyVisibility("mutate")
  @withVisibility("mutate", "create")
  model Request<Resource> {
    ...ParentKeyOf<Resource>;
    ...OmitKey<Resource>;
  }

  model Response<Resource> {
    ...KeyOf<Resource>;
    ...Changes;
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

  model Response<Resource> {
    ...Changes;
  }
}

namespace Delete {
  model Request<Resource> {
    ...KeyOf<Resource>;
    ...ParentKeyOf<Resource>;
  }

  model Response<Resource> {
    ...Changes;
  }
}
