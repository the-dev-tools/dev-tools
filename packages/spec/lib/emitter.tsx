import {
  Children,
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
  VarDeclaration,
} from '@alloy-js/typescript';
import {
  type EmitContext,
  emitFile,
  getEffectiveModelType,
  getFriendlyName,
  Interface,
  isType,
  Model,
  Namespace,
  Operation,
  Program,
  resolvePath,
  Type,
} from '@typespec/compiler';
import { writeOutput } from '@typespec/emitter-framework';
import { Array, Data, HashMap, Match, Option, pipe, Record, String } from 'effect';
import path from 'node:path';

import {
  autoChangesMap,
  baseMap,
  endpointMap,
  entityMap,
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

  pipe(
    endpointMap(program).keys(),
    Array.fromIterable,
    Array.forEach((_) => {
      const moveTo = moveMap(program).get(_);
      if (!moveTo || _.namespace === moveTo) return;
      _.namespace = moveTo;
    }),
  );
}

interface Entity {
  key: string;
  model: Model;
  primaryKeys: string[];
}

class Schema extends Data.TaggedClass('Schema')<{ type: Type }> {}

class PrimaryKeys extends Data.TaggedClass('PrimaryKeys')<{ keys: string[] }> {}

interface Endpoint {
  key: string;
  method: string;
  methodImport: string;
  operation: Operation;
  options: Record<string, PrimaryKeys | Schema>;
  service: Interface;
}

class File extends Data.TaggedClass('File')<{
  endpoints: Record<string, Endpoint>;
  entities: Record<string, Entity>;
}> {}

class Directory extends Data.TaggedClass('Directory')<{
  items: Record<string, Directory | File>;
}> {}

const getDirectory = (root: Directory, path: string[]): Directory => {
  const [pathNext, ...pathRest] = path;
  if (!pathNext) return root;

  return pipe(
    root.items[pathNext],
    Match.value,
    Match.when(undefined, () => {
      const next = new Directory({ items: {} });
      root.items[pathNext] = next;
      return getDirectory(next, pathRest);
    }),
    Match.tag('Directory', (_) => getDirectory(_, pathRest)),
    Match.tag('File', () => root),
    Match.exhaustive,
  );
};

const getFile = (root: Directory, packageName: string) => {
  const directory = getDirectory(root, packageName.split('.'));

  const filename = pipe(
    packageName,
    String.split('.'),
    Array.filter((_) => _ !== 'v1'),
    Array.last,
    Option.getOrElse(() => packageName),
  );

  let file = directory.items[filename];

  if (file === undefined) {
    file = new File({ endpoints: {}, entities: {} });
    directory.items[filename] = file;
  }

  if (file._tag !== 'File') return;

  return file;
};

const EmitContext = createContext<EmitContext>();

interface SchemaOutputProps {
  origin?: boolean;
  program: Program;
  type: Type;
}

const schemaOutput = ({ origin, program, type }: SchemaOutputProps): Option.Option<Children> => {
  if (type.kind !== 'Model') return Option.none();

  if (type.name === 'Array' && type.namespace?.name === 'TypeSpec') {
    const element = type.templateMapper?.args[0];
    if (!element || !isType(element)) return Option.none();

    return Option.map(schemaOutput({ program, type: element }), (_) => <ArrayExpression>{_}</ArrayExpression>);
  }

  if (type.namespace?.namespace?.name === 'Protobuf') return Option.none();

  if (!origin && entityMap(program).has(type)) return Option.some(refkey(type));

  return pipe(
    type.properties.values(),
    Array.fromIterable,
    Array.filterMap(({ name, type }) =>
      pipe(
        schemaOutput({ program, type }),
        Option.map((_) => <ObjectProperty name={name} value={_} />),
      ),
    ),
    Option.liftPredicate(Array.isNonEmptyArray),
    Option.map((_) => (
      <ObjectExpression>
        <CommaList>{_}</CommaList>
      </ObjectExpression>
    )),
  );
};

interface FileOutputProps {
  file: File;
  path: string;
}

const FileOutput = ({ file, path: name }: FileOutputProps) => {
  const { program } = useContext(EmitContext)!;

  const directory = useContext(SourceDirectoryContext)?.path ?? '';
  const protobuf = path.relative(directory, `../buf/typescript/${directory}/${name}_pb`);
  const dataClient = path.relative(directory, '../../data-client');

  const imports = new Map<string, Set<string>>();

  const addImport = (path: string, name: string) => {
    let pathImports = imports.get(path);
    if (!pathImports) {
      pathImports = new Set();
      imports.set(path, pathImports);
    }
    pathImports.add(name);
  };

  addImport(path.join(dataClient, 'utils'), 'makeEntity');

  Record.values(file.endpoints).forEach((_) => {
    addImport(protobuf, _.service.name);
    addImport(path.join(dataClient, _.methodImport), _.method);
  });

  Record.keys(file.entities).forEach((_) => {
    addImport(protobuf, `${_}Schema`);
  });

  return (
    <SourceFile path={`${name}.ts`}>
      <For doubleHardline each={imports.entries()}>
        {([path, imports]) => code`
          import {
            ${(
              <For comma each={Array.fromIterable(imports)} enderPunctuation>
                {(_) => _}
              </For>
            )}
          } from "${path}";
        `}
      </For>

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
                      <ObjectProperty name='message' value={`${name}Schema`} />
                      <ObjectProperty jsValue={_.key} name='key' />
                      <ObjectProperty jsValue={_.primaryKeys} name='primaryKeys' />
                      {pipe(
                        schemaOutput({ origin: true, program, type: _.model }),
                        Option.map((_) => <ObjectProperty name='schema' value={_} />),
                        Option.getOrNull,
                      )}
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

      <hardline />
      <hardline />

      <For doubleHardline each={Record.toEntries(file.endpoints)}>
        {([name, _]) => (
          <VarDeclaration const export name={`${name}Endpoint`}>
            <FunctionCallExpression
              args={[
                <ObjectExpression>
                  <CommaList>
                    {pipe(
                      Record.toEntries(_.options),
                      Array.filterMap(([name, option]) =>
                        pipe(
                          Match.value(option),
                          Match.tag('PrimaryKeys', (_) => Option.some(<ObjectProperty jsValue={_.keys} name={name} />)),
                          Match.tag('Schema', (_) =>
                            pipe(
                              schemaOutput({ program, type: _.type }),
                              Option.map((_) => <ObjectProperty name={name} value={_} />),
                            ),
                          ),
                          Match.exhaustive,
                        ),
                      ),
                      Array.prependAll([
                        <ObjectProperty
                          name='method'
                          value={`${_.service.name}.method.${String.uncapitalize(_.operation.name)}`}
                        />,
                        <ObjectProperty jsValue={_.key} name='name' />,
                      ]),
                    )}
                  </CommaList>
                </ObjectExpression>,
              ]}
              target={_.method}
            />
          </VarDeclaration>
        )}
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

  const getPackageName = (namespace?: Namespace) => {
    if (!namespace) return;
    const details = packageMap(program).get(namespace);
    const name = details?.properties.get('name')?.type;
    if (name?.kind !== 'String') return undefined;
    return name.value;
  };

  const root = new Directory({ items: {} });

  pipe(
    entityMap(program).entries(),
    Array.fromIterable,
    Array.forEach(([target, base]) => {
      const packageName = getPackageName(target.namespace);
      if (!packageName) return;

      const name = getFriendlyName(program, target) ?? target.name;
      const baseName = getFriendlyName(program, base) ?? base.name;

      const key = pipe(HashMap.get(modelKeyMap, target), Array.fromOption);
      const normalKeys = normalKeysMap(program).get(target) ?? [];
      const primaryKeys = [...key, ...normalKeys];

      const file = getFile(root, packageName);
      if (!file) return;

      file.entities[name] = {
        key: `${packageName}.${baseName}`,
        model: target,
        primaryKeys,
      };
    }),
  );

  pipe(
    endpointMap(program).entries(),
    Array.fromIterable,
    Array.forEach(([operation, meta]) => {
      if (!operation.interface) return;

      const packageName = getPackageName(operation.namespace);
      if (!packageName) return;

      const name = getFriendlyName(program, operation) ?? operation.name;

      const file = getFile(root, packageName);
      if (!file) return;

      const [methodImport, method] = meta.method.split(':');
      if (!methodImport || !method) return;

      const options = pipe(
        meta.options?.properties.entries() ?? [],
        Record.fromEntries,
        Record.filterMap(({ type: template }): Option.Option<Endpoint['options'][string]> => {
          if (template.kind !== 'Model') return Option.none();
          if (template.namespace !== program.getGlobalNamespaceType()) return Option.none();

          const type = template.templateMapper?.args[0];
          if (!type || !isType(type)) return Option.none();

          if (template.name === 'Schema') return Option.some(new Schema({ type }));

          if (template.name === 'PrimaryKeys' && type.kind === 'Model') {
            const key = pipe(HashMap.get(modelKeyMap, type), Array.fromOption);
            const normalKeys = normalKeysMap(program).get(type) ?? [];
            return Option.some(new PrimaryKeys({ keys: [...key, ...normalKeys] }));
          }

          return Option.none();
        }),
      );

      file.endpoints[name] = {
        key: `${packageName}.${operation.interface.name}/${name}`,
        method,
        methodImport,
        operation: operation,
        options,
        service: operation.interface,
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
              getPackageName(baseModel.namespace),
              Option.fromNullable,
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
