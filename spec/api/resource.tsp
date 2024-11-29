import "@typespec/protobuf";

import "../lib";

using TypeSpec.Protobuf;
using TypeSpec.Rest.Resource;

namespace API.Resource;

alias ID = bytes;

namespace List {
  @withVisibility("query", "list")
  @withDefaultKeyVisibility("query")
  model Item<Resource> {
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

  @withVisibility("query", "get")
  @withDefaultKeyVisibility("query")
  model Response<Resource> {
    ...Resource;
  }
}

namespace Create {
  @withVisibility("mutate", "create")
  @withDefaultKeyVisibility("mutate")
  model Request<Resource> {
    ...ParentKeyOf<Resource>;
    ...OmitKey<Resource>;
  }

  model Response<Resource> {
    ...KeyOf<Resource>;
  }
}

namespace Update {
  @withVisibility("mutate", "update")
  @withDefaultKeyVisibility("mutate")
  model Request<Resource extends {}> {
    ...KeyOf<Resource>;
    ...ParentKeyOf<Resource>;
    ...OptionalProperties<OmitKey<Resource>>;
  }

  model Response<Resource> {}
}

namespace Delete {
  model Request<Resource> {
    ...KeyOf<Resource>;
    ...ParentKeyOf<Resource>;
  }

  model Response<Resource> {}
}
