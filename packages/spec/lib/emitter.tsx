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
  getEffectiveModelType,
  getFriendlyName,
  Interface,
  isType,
  Model,
  Namespace,
  Operation,
  Program,
  Type,
} from '@typespec/compiler';
import { writeOutput } from '@typespec/emitter-framework';
import { Array, Data, Match, Option, pipe, Record, String } from 'effect';
import path from 'node:path';

import { endpointMap, entityMap, keyMap, messageSet, moveMap, normalKeySet, packageMap, serviceSet } from './state.js';

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

class Package extends Data.TaggedClass('Package')<{
  endpoints: Record<string, Endpoint>;
  entities: Record<string, Entity>;
}> {}

class Directory extends Data.TaggedClass('Directory')<{
  items: Record<string, Directory | Package>;
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
    Match.tag('Package', () => root),
    Match.exhaustive,
  );
};

const getPackage = (root: Directory, packageName: string) => {
  const directory = getDirectory(root, packageName.split('.'));

  const name = pipe(
    packageName,
    String.split('.'),
    Array.filter((_) => _ !== 'v1'),
    Array.last,
    Option.getOrElse(() => packageName),
  );

  let package$ = directory.items[name];

  if (package$ === undefined) {
    package$ = new Package({ endpoints: {}, entities: {} });
    directory.items[name] = package$;
  }

  if (package$._tag !== 'Package') return;

  return package$;
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

// TODO: would be much better to handle imports natively with Alloy if possible
const makeImports = () => {
  const imports = new Map<string, Set<string>>();

  const addImport = (path: string, name: string) => {
    let pathImports = imports.get(path);
    if (!pathImports) {
      pathImports = new Set();
      imports.set(path, pathImports);
    }
    pathImports.add(name);
  };

  return [imports, addImport] as const;
};

interface ImportsOutputProps {
  imports: Map<string, Set<string>>;
}

const ImportsOutput = ({ imports }: ImportsOutputProps) => (
  <>
    <For doubleHardline each={imports.entries()} ender>
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

    {code`import type * as _ from '@data-client/endpoint';`}
  </>
);

const useRoot = () => {
  const directory = useContext(SourceDirectoryContext)?.path ?? '';
  return path.relative(directory, '../..');
};

const useProtobuf = (name: string) => {
  const directory = useContext(SourceDirectoryContext)?.path ?? '';
  return path.relative(directory, `../buf/typescript/${directory}/${name}_pb.js`);
};

interface PackageOutputProps {
  name: string;
  package$: Package;
}

const EntitiesOutput = ({ name, package$ }: PackageOutputProps) => {
  const { program } = useContext(EmitContext)!;

  const root = useRoot();
  const protobuf = useProtobuf(name);

  const [imports, addImport] = makeImports();

  Record.keys(package$.entities).forEach((_) => {
    addImport(path.join(root, 'data-client/utils.js'), 'makeEntity');
    addImport(protobuf, `${_}Schema`);
  });

  return (
    <SourceFile header={<ImportsOutput imports={imports} />} path={`${name}.entities.ts`}>
      <For doubleHardline each={Record.toEntries(package$.entities)}>
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
    </SourceFile>
  );
};

const EndpointsOutput = ({ name, package$ }: PackageOutputProps) => {
  const { program } = useContext(EmitContext)!;

  const root = useRoot();
  const protobuf = useProtobuf(name);

  const [imports, addImport] = makeImports();

  Record.values(package$.endpoints).forEach((_) => {
    addImport(path.join(root, 'data-client', _.methodImport), _.method);
    addImport(protobuf, _.service.name);
  });

  return (
    <SourceFile header={<ImportsOutput imports={imports} />} path={`${name}.endpoints.ts`}>
      <For doubleHardline each={Record.toEntries(package$.endpoints)}>
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
      Match.tag('Package', (_) => (
        <>
          <EntitiesOutput name={path} package$={_} />
          <EndpointsOutput name={path} package$={_} />
        </>
      )),
      Match.exhaustive,
    ),
  );

  return <SourceDirectory path={path}>{Record.values(items)}</SourceDirectory>;
};

export async function $onEmit(context: EmitContext) {
  moveMessages(context);

  const { emitterOutputDir, program } = context;

  if (program.compilerOptions.noEmit) return;

  const getPrimaryKeys = (model: Model): string[] =>
    pipe(
      model.properties.values(),
      Array.fromIterable,
      Array.flatMapNullable((_) => {
        if (keyMap(program).has(_)) return _.name;
        if (normalKeySet(program).has(_)) return _.name;
        return undefined;
      }),
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
      const primaryKeys = getPrimaryKeys(target);

      const package$ = getPackage(root, packageName);
      if (!package$) return;

      package$.entities[name] = {
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

      const package$ = getPackage(root, packageName);
      if (!package$) return;

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

          if (template.name === 'PrimaryKeys' && type.kind === 'Model')
            return Option.some(new PrimaryKeys({ keys: getPrimaryKeys(type) }));

          return Option.none();
        }),
      );

      package$.endpoints[name] = {
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
}
