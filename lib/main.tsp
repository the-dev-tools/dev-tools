import "@typespec/protobuf";

import "./lib.js";

using TypeSpec.Reflection;

extern dec copyKey(target: Model);

@copyKey
model KeyOf<Resource> {}

extern dec copyParentKey(target: Model);

@copyParentKey
model ParentKeyOf<Resource> {}

extern dec omitKey(target: Model);

@omitKey
model OmitKey<Resource> {
  ...Resource;
}

extern dec autoField(target: ModelProperty);
extern dec autoFields(target: Model);
