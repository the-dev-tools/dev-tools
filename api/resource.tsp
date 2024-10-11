using TypeSpec.Protobuf;
using TypeSpec.Rest.Resource;

namespace API.Resource;

namespace List {
  @withVisibility("query", "list")
  model Item<Resource> {
    ...Resource;
  }

  @friendlyName("List{name}Request", Resource)
  model Request<Resource> {
    ...ParentKeysOf<Resource>;
  }

  @friendlyName("List{name}Response", Resource)
  model Response<Resource, Item> {
    @field(1) items: Item[];
  }
}

namespace Get {
  @friendlyName("Get{name}Request", Resource)
  model Request<Resource> {
    ...KeysOf<Resource>;
  }

  @friendlyName("Get{name}Response", Resource)
  @withVisibility("query", "get")
  model Response<Resource> {
    ...Resource;
  }
}

namespace Create {
  @friendlyName("Create{name}Request", Resource)
  model Request<Resource extends {}> {
    ...ResourceCreateModel<Resource>;
  }

  @friendlyName("Create{name}Response", Resource)
  model Response<Resource> {
    ...KeysOf<Resource>;
  }
}

namespace Update {
  @friendlyName("Update{name}Request", Resource)
  model Request<Resource extends {}> {
    ...KeysOf<Resource>;
    ...ResourceCreateOrUpdateModel<Resource>;
  }

  @friendlyName("Update{name}Response", Resource)
  model Response<Resource> {}
}

namespace Delete {
  @friendlyName("Delete{name}Request", Resource)
  model Request<Resource> {
    ...KeysOf<Resource>;
  }

  @friendlyName("Delete{name}Response", Resource)
  model Response<Resource> {}
}
