import "@typespec/protobuf";

import "../api/change.tsp";

import "./index.js";

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

extern dec normalize(target: Model, base?: Model);
extern dec normalKey(target: ModelProperty);

extern dec autoChange(target: TypeSpec.Reflection.Model, change: valueof API.Change.AutoChange);
