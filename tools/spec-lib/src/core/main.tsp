import "../../dist/src/core";

namespace DevTools;

using Reflection;

extern dec project(target: Namespace, version?: int8);

extern dec primaryKey(target: ModelProperty);
extern dec foreignKey(target: ModelProperty);

enum KeyAs {
  Primary,
  Foreign,
  None,
  Omit,
}

model KeysAs {
  primary?: KeyAs;
  foreign?: KeyAs;
}

@Private.copyKeys(TResource, TKeysAs)
model Keys<TResource extends Model, TKeysAs extends valueof KeysAs = #{}> {}

namespace Private {
  extern dec copyKeys(target: Model, source: Model, keysAs: valueof KeysAs);
}

extern dec withDelta(target: Model);

@project
namespace Global {
  scalar UnsetDelta;
}
