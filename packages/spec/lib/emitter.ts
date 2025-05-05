import {
  EmitContext,
  emitFile,
  getEffectiveModelType,
  getFriendlyName,
  Interface,
  isType,
  Model,
  Namespace,
  resolvePath,
  Type,
} from '@typespec/compiler';
import { Array, HashMap, Option, pipe, Record } from 'effect';

import { $lib } from './lib.js';

const protobufState = {
  message: Symbol.for('@typespec/protobuf.message'),
  package: Symbol.for('@typespec/protobuf.package'),
  service: Symbol.for('@typespec/protobuf.service'),
};

function moveMessages({ program }: EmitContext) {
  // Get declared packages
  const packages = pipe(
    program.stateMap(protobufState.package) as Map<Namespace, Model>,
    (_) => _.keys(),
    Array.fromIterable,
  );

  // Get declared services
  const services = program.stateSet(protobufState.service) as Set<Interface>;

  // Get declared messages
  const messages = new Set(program.stateSet(protobufState.message)) as Set<Model>;

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

      if (_.returnType.kind === 'Model') messages.add(_.returnType);
    }),
  );

  function addNestedMessages(model: Model) {
    messages.add(model);

    if (model.name === 'Array' && model.namespace?.name === 'TypeSpec') {
      const type = model.templateMapper?.args[0];
      if (!type || !isType(type) || type.kind !== 'Model' || messages.has(type)) return;
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

  const moves = program.stateMap($lib.stateKeys.move) as Map<Model, Namespace>;

  Array.fromIterable(messages).forEach((_) => {
    const moveTo = moves.get(_);
    if (!moveTo || _.namespace === moveTo) return;

    const name = getFriendlyName(program, _) ?? _.name;
    _.namespace = moveTo;
    moveTo.models.set(name, _);
  });
}

export async function $onEmit(context: EmitContext) {
  moveMessages(context);

  const { emitterOutputDir, program } = context;

  if (program.compilerOptions.noEmit) return;

  const modelKeyMap = pipe(
    program.stateMap(Symbol.for('TypeSpec.key')) as Map<Type, string>,
    (_) => _.entries(),
    Array.fromIterable,
    Array.filterMap(([type, key]) => {
      if (type.kind !== 'ModelProperty') return Option.none();
      return Option.some([type.model, key] as const);
    }),
    HashMap.fromIterable,
  );

  const packageStateMap = program.stateMap(protobufState.package) as Map<Namespace, Model>;

  const getPackageName = (details: Model) => {
    const name = details.properties.get('name')?.type;
    if (name?.kind !== 'String') return null;
    return name.value;
  };

  const typeMap = pipe(
    packageStateMap.entries(),
    Array.fromIterable,
    Array.flatMapNullable(([{ models }, details]) => {
      const packageName = details.properties.get('name')?.type;
      if (packageName?.kind !== 'String') return null;

      return pipe(
        models.values(),
        Array.fromIterable,
        Array.map((model) => {
          const name = getFriendlyName(program, model) ?? model.name;
          const typeName = `${packageName.value}.${name}`;

          let meta: Record<string, unknown> = {
            autoChanges: (program.stateMap($lib.stateKeys.autoChanges) as Map<Model, unknown>).get(model),
          };

          const baseModel = (program.stateMap($lib.stateKeys.base) as Map<Model, Model>).get(model);
          if (baseModel) {
            pipe(
              Option.fromNullable(baseModel.namespace),
              Option.flatMapNullable((_) => packageStateMap.get(_)),
              Option.flatMapNullable(getPackageName),
              Option.map((_) => {
                meta = { ...meta, base: `${_}.${baseModel.name}` };
              }),
            );
          }

          if (baseModel === model) {
            meta = {
              ...meta,
              key: pipe(HashMap.get(modelKeyMap, model), Option.getOrUndefined),
              normalKeys: program.stateMap($lib.stateKeys.normalKeys).get(model),
            };
          }

          return [typeName, meta] as const;
        }),
      );
    }),
    Array.flatten,
    Record.fromEntries,
  );

  await emitFile(program, {
    content: JSON.stringify(typeMap, undefined, 2),
    path: resolvePath(emitterOutputDir, 'meta.json'),
  });
}
