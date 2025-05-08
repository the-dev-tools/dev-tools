/* eslint-disable react/jsx-key */
import { code, For, Output, refkey, SourceDirectory, SourceDirectoryContext, useContext } from '@alloy-js/core';
import {
  ClassDeclaration,
  CommaList,
  FunctionCallExpression,
  ObjectExpression,
  ObjectProperty,
  SourceFile,
} from '@alloy-js/typescript';
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
import { writeOutput } from '@typespec/emitter-framework';
import { Array, Data, HashMap, Match, Option, pipe, Record, String } from 'effect';
import path from 'node:path';

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

  // const keyMap = program.stateMap(Symbol.for('TypeSpec.key')) as Map<Type, string>;
  const normalKeysMap = program.stateMap($lib.stateKeys.normalKeys) as Map<Model, string[]>;

  const packageStateMap = program.stateMap(protobufState.package) as Map<Namespace, Model>;

  const getPackageName = (details: Model) => {
    const name = details.properties.get('name')?.type;
    if (name?.kind !== 'String') return null;
    return name.value;
  };

  const bases = program.stateMap($lib.stateKeys.base) as Map<Model, Model>;

  interface Entity {
    key: string;
    model: Model;
    primaryKeys: string[];
  }

  class File extends Data.TaggedClass('File')<{
    entities: Record<string, Entity>;
  }> {}

  class Directory extends Data.TaggedClass('Directory')<{
    items: Record<string, Directory | File>;
  }> {}

  const root = new Directory({ items: {} });

  const getOrMakeDirectory = (root: Directory, path: string[]): Directory => {
    const [pathNext, ...pathRest] = path;
    if (!pathNext) return root;

    return pipe(
      // root.files.get(pathNext),
      root.items[pathNext],
      Match.value,
      Match.when(undefined, () => {
        const next = new Directory({ items: {} });
        root.items[pathNext] = next;
        // root.files.set(pathNext, next);
        return getOrMakeDirectory(next, pathRest);
      }),
      Match.tag('Directory', (_) => getOrMakeDirectory(_, pathRest)),
      Match.tag('File', () => root),
      Match.exhaustive,
    );
  };

  pipe(
    bases.entries(),
    Array.fromIterable,
    Array.forEach(([target, base]) => {
      if (!target.namespace) return;
      const packageName = packageStateMap.get(target.namespace)?.properties.get('name')?.type;
      if (packageName?.kind !== 'String') return;

      const name = getFriendlyName(program, target) ?? target.name;
      const baseName = getFriendlyName(program, base) ?? base.name;

      const key = pipe(HashMap.get(modelKeyMap, target), Array.fromOption);
      const normalKeys = normalKeysMap.get(target) ?? [];
      const primaryKeys = [...key, ...normalKeys];

      const directory = getOrMakeDirectory(root, packageName.value.split('.'));

      const filename = pipe(
        packageName.value,
        String.split('.'),
        Array.filter((_) => _ !== 'v1'),
        Array.last,
        Option.getOrElse(() => packageName.value),
      );

      let file = directory.items[filename];

      if (file === undefined) {
        file = new File({ entities: {} });
        directory.items[filename] = file;
      }

      if (file._tag !== 'File') return;

      file.entities[name] = {
        key: `${packageName.value}.${baseName}`,
        model: target,
        primaryKeys,
      };
    }),
  );

  const FileOutput = ({ file, path: name }: { file: File; path: string }) => {
    const directory = useContext(SourceDirectoryContext)?.path ?? '';
    const protobuf = path.relative(directory, `../buf/typescript/${directory}/${name}_pb`);
    const dataClient = path.relative(directory, '../../data-client');

    return (
      <SourceFile path={`${name}.ts`}>
        {code`
          import { makeEntity } from "${dataClient}/utils";

          import {
            ${(
              <For comma each={Record.keys(file.entities)} enderPunctuation hardline>
                {(_) => `${_}Schema`}
              </For>
            )}
          } from "${protobuf}";
        `}

        <hardline />
        <hardline />

        <For doubleHardline each={Record.toEntries(file.entities)}>
          {([name, _]) => (
            <ClassDeclaration
              export
              extends={
                <FunctionCallExpression
                  args={[
                    <ObjectExpression>
                      <CommaList>
                        <ObjectProperty name='schema' value={`${name}Schema`} />
                        <ObjectProperty jsValue={_.key} name='key' />
                        <ObjectProperty jsValue={_.primaryKeys} name='primaryKeys' />
                      </CommaList>
                    </ObjectExpression>,
                  ]}
                  target='makeEntity'
                />
              }
              name={`${name}Entity`}
              refkey={refkey(_.model)}
            />
          )}
        </For>
      </SourceFile>
    );
  };

  const DirectoryOutput = ({ directory, path }: { directory: Directory; path: string }) => {
    const items = Record.map(directory.items, (item, path) =>
      pipe(
        Match.value(item),
        Match.tag('Directory', (_) => <DirectoryOutput directory={_} path={path} />),
        Match.tag('File', (_) => <FileOutput file={_} path={path} />),
        Match.exhaustive,
      ),
    );

    return <SourceDirectory path={path}>{Record.values(items)}</SourceDirectory>;
  };

  await writeOutput(
    program,
    <Output>
      <DirectoryOutput directory={root} path='.' />
    </Output>,
    emitterOutputDir,
  );

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
