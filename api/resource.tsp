import "@typespec/protobuf";
import "@typespec/rest";

using TypeSpec.Protobuf;
using TypeSpec.Rest.Resource;

namespace API.Resource;

namespace List {
  @withVisibility("query", "list")
  model Item<Resource> {
    ...Resource;
  }

  model Request<Resource> {
    ...ParentKeysOf<Resource>;
  }

  model Response<Item> {
    @field(1) items: Item[];
  }
}

namespace Get {
  model Request<Resource> {
    ...KeysOf<Resource>;
  }

  @withVisibility("query", "get")
  model Response<Resource> {
    ...Resource;
  }
}

namespace Create {
  model Request<Resource extends {}> {
    ...ResourceCreateModel<Resource>;
  }

  model Response<Resource> {
    ...KeysOf<Resource>;
  }
}

namespace Update {
  model Request<Resource extends {}> {
    ...KeysOf<Resource>;
    ...ResourceCreateOrUpdateModel<Resource>;
  }

  model Response<Resource> {}
}

namespace Delete {
  model Request<Resource> {
    ...KeysOf<Resource>;
  }

  model Response<Resource> {}
}
