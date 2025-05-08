import {
  code,
  createContext,
  For,
  Output,
  refkey,
  SourceDirectory,
  SourceDirectoryContext,
  useContext,
} from '@alloy-js/core';
import {
  ArrayExpression,
  ClassDeclaration,
  CommaList,
  FunctionCallExpression,
  ObjectExpression,
  ObjectProperty,
  SourceFile,
} from '@alloy-js/typescript';
import {
  type EmitContext,
  emitFile,
  getEffectiveModelType,
  getFriendlyName,
  isType,
  Model,
  resolvePath,
} from '@typespec/compiler';
import { writeOutput } from '@typespec/emitter-framework';
import { Array, Data, HashMap, Match, Option, pipe, Record, String } from 'effect';
import path from 'node:path';

import {
  autoChangesMap,
  baseMap,
  keyMap,
  messageSet,
  moveMap,
  normalKeysMap,
  packageMap,
  serviceSet,
} from './state.js';

function moveMessages({ program }: EmitContext) {
  // Get declared packages
  const packages = pipe(packageMap(program).keys(), Array.fromIterable);

  // Get declared services
  const services = new Set(serviceSet(program));

  // Get declared messages
  const messages = new Set(messageSet(program));

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

  Array.fromIterable(messages).forEach((_) => {
    const moveTo = moveMap(program).get(_);
    if (!moveTo || _.namespace === moveTo) return;

    const name = getFriendlyName(program, _) ?? _.name;
    _.namespace = moveTo;
    moveTo.models.set(name, _);
  });
}

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

const EmitContext = createContext<EmitContext>();

interface FileOutputProps {
  file: File;
  path: string;
}

const FileOutput = ({ file, path: name }: FileOutputProps) => {
  const { program } = useContext(EmitContext)!;

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
        {([name, _]) => {
          const schema = pipe(
            _.model.properties.values(),
            Array.fromIterable,
            Array.filterMap((_) => {
              if (_.type.kind !== 'Model') return Option.none();

              if (_.type.name === 'Array' && _.type.namespace?.name === 'TypeSpec') {
                const type = _.type.templateMapper?.args[0];

                if (!type || !isType(type) || type.kind !== 'Model' || !baseMap(program).has(type)) {
                  return Option.none();
                }

                return Option.some(
                  <ObjectProperty name={_.name} value={<ArrayExpression>{refkey(type)}</ArrayExpression>} />,
                );
              }

              if (!baseMap(program).has(_.type)) return Option.none();
              return Option.some(<ObjectProperty name={_.name} value={refkey(_.type)} />);
            }),
            Option.liftPredicate(Array.isNonEmptyArray),
            Option.map((_) => (
              <ObjectExpression>
                <CommaList>{_}</CommaList>
              </ObjectExpression>
            )),
            Option.getOrNull,
          );

          return (
            <ClassDeclaration
              export
              extends={
                <FunctionCallExpression
                  args={[
                    <ObjectExpression>
                      <CommaList>
                        <ObjectProperty name='message' value={`${name}Schema`} />
                        <ObjectProperty jsValue={_.key} name='key' />
                        <ObjectProperty jsValue={_.primaryKeys} name='primaryKeys' />
                        {schema && <ObjectProperty name='schema' value={schema} />}
                      </CommaList>
                    </ObjectExpression>,
                  ]}
                  target='makeEntity'
                />
              }
              name={`${name}Entity`}
              refkey={refkey(_.model)}
            />
          );
        }}
      </For>
    </SourceFile>
  );
};

interface DirectoryOutputProps {
  directory: Directory;
  path: string;
}

const DirectoryOutput = ({ directory, path }: DirectoryOutputProps) => {
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

export async function $onEmit(context: EmitContext) {
  moveMessages(context);

  const { emitterOutputDir, program } = context;

  if (program.compilerOptions.noEmit) return;

  const modelKeyMap = pipe(
    keyMap(program),
    (_) => _.entries(),
    Array.fromIterable,
    Array.filterMap(([type, key]) => {
      if (type.kind !== 'ModelProperty') return Option.none();
      return Option.some([type.model, key] as const);
    }),
    HashMap.fromIterable,
  );

  const getPackageName = (details: Model) => {
    const name = details.properties.get('name')?.type;
    if (name?.kind !== 'String') return null;
    return name.value;
  };

  const root = new Directory({ items: {} });

  pipe(
    baseMap(program).entries(),
    Array.fromIterable,
    Array.forEach(([target, base]) => {
      if (!target.namespace) return;
      const packageName = packageMap(program).get(target.namespace)?.properties.get('name')?.type;
      if (packageName?.kind !== 'String') return;

      const name = getFriendlyName(program, target) ?? target.name;
      const baseName = getFriendlyName(program, base) ?? base.name;

      const key = pipe(HashMap.get(modelKeyMap, target), Array.fromOption);
      const normalKeys = normalKeysMap(program).get(target) ?? [];
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

  await writeOutput(
    program,
    <Output>
      <EmitContext.Provider value={context}>
        <DirectoryOutput directory={root} path='.' />
      </EmitContext.Provider>
    </Output>,
    emitterOutputDir,
  );

  const typeMap = pipe(
    packageMap(program).entries(),
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
            autoChanges: autoChangesMap(program).get(model),
          };

          const baseModel = baseMap(program).get(model);
          if (baseModel) {
            pipe(
              Option.fromNullable(baseModel.namespace),
              Option.flatMapNullable((_) => packageMap(program).get(_)),
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
              normalKeys: normalKeysMap(program).get(model),
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
