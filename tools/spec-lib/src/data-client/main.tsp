import "../../dist/src/data-client";

namespace DevTools.DataClient;

using Reflection;

extern dec entity(target: Model, base?: Model);

extern dec endpoint(target: Operation, method: string, options?: Model);

model Schema<Resource extends Model> {}
model PrimaryKeys<Resource extends Model> {}
