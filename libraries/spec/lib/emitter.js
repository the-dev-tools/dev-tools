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

            /** @type {Model | undefined} */
            const base = program.stateMap($lib.stateKeys.base).get(model);

            if (!base) return Option.none();

            if (base !== model) {
              const hasBase = program.stateMap($lib.stateKeys.base).get(base) === base;
              if (!hasBase) return Option.none();

              /** @type {string} */
              const basePackageName = packageStateMap.get(base.namespace).properties.get('name').type.value;
              const baseTypeName = `${basePackageName}.${base.name}`;
              return Option.some(/** @type {const} */ ([typeName, { base: baseTypeName }]));
            }

            const key = pipe(HashMap.get(modelKeyMap, model), Option.getOrUndefined);

            /** @type {string[] | undefined} */
            const normalKeys = program.stateMap($lib.stateKeys.normalKeys).get(model);

            return Option.some(/** @type {const} */ ([typeName, { key, normalKeys }]));
          }),
        );
      },
    ),
    Array.getSomes,
    Record.fromEntries,
  );

  await emitFile(program, {
    path: resolvePath(emitterOutputDir, 'meta.json'),
    content: JSON.stringify(typeMap, undefined, 2),
  });
}
