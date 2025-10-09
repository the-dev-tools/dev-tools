import "../../dist/src/core";

namespace DevTools;

using Reflection;

extern dec parent(target: Model, parent: Model);

extern dec copyParent(target: Model, base: Model);

extern dec normalKey(target: ModelProperty);

extern dec project(target: Namespace, version?: int8);

@Private.copyKey(Source)
model KeyOf<Source extends Model> {}

@Private.copyParentKey(Source)
model ParentKeyOf<Source extends Model> {}

@Private.omitKey
model OmitKey<Resource> {
  ...Resource;
}

namespace Private {
  extern dec copyKey(target: Model, source: Model);
  extern dec copyParentKey(target: Model, base: Model);
  extern dec omitKey(target: Model);
}
