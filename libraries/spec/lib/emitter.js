import { createTypeSpecLibrary, emitFile, resolvePath } from '@typespec/compiler';
import { Array, HashMap, Option, pipe, Record } from 'effect';

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
          Array.map((model) =>
            pipe(
              HashMap.get(keys, model),
              Option.map(
                /** @returns {[string, string]} */
                (key) => [`${packageName}.${model.name}`, key],
              ),
            ),
          ),
        );
      },
    ),
    Array.getSomes,
    Record.fromEntries,
  );

  await emitFile(program, {
    path: resolvePath(emitterOutputDir, 'message-id-map.json'),
    content: JSON.stringify(messageIdMap, undefined, 2),
  });
}

export const $lib = createTypeSpecLibrary({
  name: 'meta',
  diagnostics: {},
});
