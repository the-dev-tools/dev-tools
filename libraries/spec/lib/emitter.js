import { createTypeSpecLibrary, emitFile, resolvePath } from '@typespec/compiler';
import { Array, HashMap, Option, pipe, Record } from 'effect';

import { $lib } from './lib.js';

/** @import { EmitContext, Model, Namespace } from '@typespec/compiler' */

/** @param {EmitContext} context */
export async function $onEmit({ program, emitterOutputDir }) {
  if (program.compilerOptions.noEmit) return;

  const keys = pipe(
    program.stateMap(Symbol.for('TypeSpec.key')).entries(),
    Array.fromIterable,
    Array.map(
      /** @returns {[Model, string]} */
      ([type, key]) => [type.model, key],
    ),
    HashMap.fromIterable,
  );

  const messageIdMap = pipe(
    program.stateMap(Symbol.for('@typespec/protobuf.package')).entries(),
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

            const key = pipe(HashMap.get(keys, model), Option.getOrUndefined);

            /** @type {string[]} */
            let normalKeys = program.stateMap($lib.stateKeys.normalKey).get(model) ?? [];
            if (key) normalKeys.unshift(key);
            if (!normalKeys.length) normalKeys = undefined;

            return [typeName, { key, normalKeys }];
          }),
        );
      },
    ),
    Record.fromEntries,
  );

  await emitFile(program, {
    path: resolvePath(emitterOutputDir, 'message-id-map.json'),
    content: JSON.stringify(messageIdMap, undefined, 2),
  });
}
