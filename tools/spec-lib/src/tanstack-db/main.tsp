import "../core";
import "../../dist/src/tanstack-db";

namespace DevTools.TanStackDB {
  model CollectionOptions {
    isReadOnly?: boolean = false;
    canCreate?: boolean = true;
    canUpdate?: boolean = true;
    canDelete?: boolean = true;
  }

  extern dec collection(target: Reflection.Model, options?: valueof CollectionOptions);
}

@DevTools.project
namespace DevTools.Global {
  scalar Unset;
}
