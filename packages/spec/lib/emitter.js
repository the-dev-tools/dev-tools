import { emitFile, getEffectiveModelType, getFriendlyName, isType, resolvePath } from '@typespec/compiler';
import { Array, HashMap, Option, pipe, Record } from 'effect';

import { $lib } from './lib.js';

/** @import { EmitContext, Model, Namespace, Interface } from '@typespec/compiler' */

const protobufState = {
  package: Symbol.for('@typespec/protobuf.package'),
  service: Symbol.for('@typespec/protobuf.service'),
  message: Symbol.for('@typespec/protobuf.message'),
};

/** @param {EmitContext} context */
function moveMessages({ program }) {
  // Get declared packages
  /** @type {Array<Namespace>} */
  const packages = pipe(program.stateMap(protobufState.package).keys(), Array.fromIterable);

  // Get declared services
  /** @type {Set<Interface>} */
  const services = program.stateSet(protobufState.service);

  // Get declared messages
  /** @type {Set<Model>} */
  const messages = new Set(program.stateSet(protobufState.message));

  // Get package messages and services
  packages.forEach((_) => {
    pipe(
      _.models.values(),
      Array.fromIterable,
      Array.forEach((_) => messages.add(_)),
    );

    pipe(
      _.interfaces.values(),
      Array.fromIterable,
      Array.forEach((_) => services.add(_)),
    );
  });

  // Get service messages
  pipe(
    Array.fromIterable(services),
    Array.flatMap((_) => pipe(_.operations.values(), Array.fromIterable)),
    Array.forEach((_) => {
      if (_.parameters.properties.size !== 0)
        pipe(getEffectiveModelType(program, _.parameters), (_) => messages.add(_));

      messages.add(_.returnType);
    }),
  );

  /** @param {Model} model */
  function addNestedMessages(model) {
    messages.add(model);

    if (model.name === 'Array' && model.namespace?.name === 'TypeSpec') {
      const type = model.templateMapper.args[0];
      if (!isType(type)) return;
      if (type.kind !== 'Model') return;
      if (messages.has(type)) return;
      addNestedMessages(type);
    }

    pipe(
      model.properties.values(),
      Array.fromIterable,
      Array.forEach((_) => {
        if (_.type.kind !== 'Model') return;
        if (messages.has(_.type)) return;
        addNestedMessages(_.type);
      }),
    );
  }

  pipe(Array.fromIterable(messages), Array.forEach(addNestedMessages));

  /** @type {Map<Model, Namespace>} */
  const moves = program.stateMap($lib.stateKeys.move);

  Array.fromIterable(messages).forEach((_) => {
    const moveTo = moves.get(_);
    if (!moveTo || _.namespace === moveTo) return;

    const name = getFriendlyName(program, _) ?? _.name;
    _.namespace = moveTo;
    moveTo.models.set(name, _);
  });
}

/** @param {EmitContext} context */
export async function $onEmit(context) {
  moveMessages(context);

  const { program, emitterOutputDir } = context;

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

  const packageStateMap = program.stateMap(protobufState.package);

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
            const name = getFriendlyName(program, model) ?? model.name;
            const typeName = `${packageName}.${name}`;

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
