import "../../dist/src/core";

namespace DevTools;

using Reflection;

extern dec instanceOf(instance: Model, template: Model);

extern dec templateOf(template: Model | Operation | Interface, base?: Model);
