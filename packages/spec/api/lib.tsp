import "@typespec/protobuf";

import "../dist/lib/index.js";

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

/** Move model to specified namespace. Useful for collocating templated models */
extern dec move(target: Model | Operation, from: Namespace | Model, to: Namespace | Model);

/**
 * Replace name with friendly name. Useful for when friendly names are
 * unsupported, f.e. operations in Protobuf emitter
 */
extern dec useFriendlyName(target: Operation | Model);

extern dec entity(target: Model, base?: Model);

extern dec endpoint(target: Operation, method: string, options?: Model);

model Schema<Resource extends Model> {}
model PrimaryKeys<Resource extends Model> {}

/**
 * Workaround to allow comments before first interface
 */
interface BlankInterface {}
