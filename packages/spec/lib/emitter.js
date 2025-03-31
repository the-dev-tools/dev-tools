import { createTypeSpecLibrary, emitFile, resolvePath } from '@typespec/compiler';
import { Array, HashMap, Option, pipe, Record } from 'effect';

import { $lib } from './lib.js';

/** @import { EmitContext, Model, Namespace } from '@typespec/compiler' */

/** @param {EmitContext} context */
export async function $onEmit({ program, emitterOutputDir }) {
  if (program.compilerOptions.noEmit) return;

  const modelKeyMap = pipe(
    program.stateMap(Symbol.for('TypeSpec.key')).entries(),
    Array.fromIterable,
    Array.map(
      /** @returns {[Model, string]} */
      ([type, key]) => [type.model, key],
    ),
    HashMap.fromIterable,
  );

  const packageStateMap = program.stateMap(Symbol.for('@typespec/protobuf.package'));

  const typeMap = pipe(
    packageStateMap.entries(),
    Array.fromIterable,
    Array.flatMap(
      /** @param {[Namespace, Model]} entry */
      ([{ models }, details]) => {
        /** @type {string} */
        const packageName = details.properties.get('name').type.value;

        return pipe(
          models.values(),
          Array.fromIterable,
          Array.map((model) => {
            const typeName = `${packageName}.${model.name}`;

            let meta = { autoChanges: program.stateMap($lib.stateKeys.autoChanges).get(model) };

            /** @type {Model | undefined} */
            let baseModel = program.stateMap($lib.stateKeys.base).get(model);
            if (baseModel) {
              const basePackageName = packageStateMap.get(baseModel.namespace).properties.get('name').type.value;
              meta = { ...meta, base: `${basePackageName}.${baseModel.name}` };
            }

            if (baseModel === model) {
              meta = {
                ...meta,
                key: pipe(HashMap.get(modelKeyMap, model), Option.getOrUndefined),
                normalKeys: program.stateMap($lib.stateKeys.normalKeys).get(model),
              };
            }

            return /** @type {const} */ ([typeName, meta]);
          }),
        );
      },
    ),
    Record.fromEntries,
  );

  await emitFile(program, {
    path: resolvePath(emitterOutputDir, 'meta.json'),
    content: JSON.stringify(typeMap, undefined, 2),
  });
}
