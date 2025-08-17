import {
  Binder,
  Children,
  createContext,
  For,
  getSymbolCreatorSymbol,
  Refkey,
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
  TSModuleScope,
  TSOutputSymbol,
  TSPackageScope,
  VarDeclaration,
} from '@alloy-js/typescript';
import {
  EmitContext,
  isKey,
  isTemplateDeclaration,
  Model,
  Namespace,
  Operation,
  Program,
  Type,
} from '@typespec/compiler';
import { $ } from '@typespec/compiler/typekit';
import { Output, useTsp, writeOutput } from '@typespec/emitter-framework';
import { Array, flow, HashMap, Option, pipe, Predicate, Schema, String, Tuple } from 'effect';
import { join } from 'node:path/posix';
import {
  EmitterOptions,
  getModelDerivations,
  getModelName,
  getModelNamespace,
  getModelProperties,
  getModelRefKey,
  normalKeys,
} from '../core/index.js';
import { EndpointMeta, endpoints, entities } from './lib.js';

const EmitterOptionsContext = createContext<EmitterOptions>();

export const $onEmit = async (context: EmitContext<(typeof EmitterOptions)['Encoded']>) => {
  const { emitterOutputDir, program } = context;

  const options = Schema.decodeSync(EmitterOptions)(context.options);

  if (program.compilerOptions.noEmit) return;

  const root = program.getGlobalNamespaceType().namespaces.get(options.rootNamespace);
  if (!root) return;

  const bindExternals = (binder: Binder) => {
    const scopes = new Map<string, TSModuleScope>();

    const outputSymbol = (path: string, name: string, refkeys: Refkey) => {
      const scope = scopes.get(path) ?? new TSModuleScope(path, { parent: binder.globalScope });
      scopes.set(path, scope);
      binder.notifyScopeCreated(scope);

      if (scope.exportedSymbols.has(refkeys)) return;
      const symbol = new TSOutputSymbol(name, { refkeys, scope });
      scope.exportedSymbols.set(refkeys, symbol);
      binder.notifySymbolCreated(symbol);
    };

    const getProtobufPath = (namespace?: Namespace) => {
      const getNamespacePath = (namespace?: Namespace): Namespace[] => {
        if (!namespace || namespace === root) return [];
        return pipe(getNamespacePath(namespace.namespace), Array.append(namespace));
      };

      const namespacePath = pipe(
        getNamespacePath(namespace),
        Array.map((_) => String.pascalToSnake(_.name)),
      );

      return join(
        options.bufTypeScriptPath,
        ...namespacePath,
        `v${options.version}`,
        pipe(Array.last(namespacePath), Option.getOrThrow, (_) => `${_}_pb.js`),
      );
    };

    const libPackageScope = new TSPackageScope('@the-dev-tools/spec-lib', '', '', { parent: binder.globalScope });
    const utilsScope = new TSModuleScope('', { parent: libPackageScope });

    const addUtil = (name: string) => {
      const refkeys = refkey(name);
      const symbol = new TSOutputSymbol(name, { refkeys, scope: utilsScope });
      utilsScope.exportedSymbols.set(refkeys, symbol);
      binder.notifySymbolCreated(symbol);
    };

    binder.notifyScopeCreated(libPackageScope);
    libPackageScope.addExport('data-client/utils.js', utilsScope);
    addUtil('makeEntity');

    pipe(
      entities(program).entries().toArray(),
      Array.filter(([_]) => !isTemplateDeclaration(_)),
      Array.forEach(([model, base]) => {
        const namespace = getModelNamespace(program, base);
        const path = getProtobufPath(namespace);
        const name = getModelName(program, model);
        const key = getModelRefKey(program, model);
        outputSymbol(path, name + 'Schema', refkey('schema', key));
      }),
    );

    pipe(
      endpoints(program).entries().toArray(),
      Array.filter(([_]) => {
        $(program).type.finishType(_);
        return !isTemplateDeclaration(_);
      }),
      Array.forEach(([operation, meta]) => {
        const interface_ = operation.interface;
        if (!interface_) return;

        const [methodPathRelative, method] = pipe(
          meta.method.split(':'),
          Option.liftPredicate(Tuple.isTupleOf(2)),
          Option.getOrThrow,
        );

        const methodPath = join(options.dataClientPath, methodPathRelative);
        outputSymbol(methodPath, method, refkey('method', meta.method));

        const path = getProtobufPath(interface_.namespace);
        outputSymbol(path, interface_.name, refkey('service', interface_));
      }),
    );
  };

  await writeOutput(
    program,
    <EmitterOptionsContext.Provider value={options}>
      <Output externals={[{ [getSymbolCreatorSymbol()]: bindExternals }]} program={program}>
        {pipe(
          root.namespaces.values().toArray(),
          Array.map((_) => <Directory namespace={_} />),
        )}
      </Output>
    </EmitterOptionsContext.Provider>,
    join(emitterOutputDir, 'data-client'),
  );
};

interface DirectoryProps {
  namespace: Namespace;
}

const Directory = ({ namespace }: DirectoryProps) => {
  const { program } = useTsp();
  const { version } = useContext(EmitterOptionsContext)!;

  const name = String.pascalToSnake(namespace.name);

  // const parent = useContext(SourceDirectoryContext)?.path;

  // let path = `${name}/v${version}`;
  // if (parent && parent !== './') path = `${parent}/${path}`;

  const subdirectories = pipe(
    namespace.namespaces.values(),
    Array.fromIterable,
    Array.map((_) => <Directory namespace={_} />),
  );

  const entityFile = pipe(
    namespace.models.values().toArray(),
    Array.flatMap((_) => getModelDerivations(program, _)),
    Array.dedupe,
    Array.filter((_) => entities(program).has(_)),
    (_) => (
      <SourceFile path={`${name}.entities.ts`}>
        <For doubleHardline each={_}>
          {(_) => <Entity model={_} />}
        </For>
      </SourceFile>
    ),
  );

  const endpointsFile = pipe(
    namespace.interfaces.values().toArray(),
    Array.flatMap((_) => _.operations.values().toArray()),
    Array.filterMap((operation) =>
      pipe(
        endpoints(program).get(operation),
        Option.fromNullable,
        Option.map((meta) => ({ meta, operation })),
      ),
    ),
    (_) => (
      <SourceFile path={`${name}.endpoints.ts`}>
        <For doubleHardline each={_}>
          {({ meta, operation }) => <Endpoint meta={meta} operation={operation} />}
        </For>
      </SourceFile>
    ),
  );

  return (
    <SourceDirectory path={name}>
      {subdirectories}
      <SourceDirectory path={`v${version}`}>
        {entityFile}
        {endpointsFile}
      </SourceDirectory>
    </SourceDirectory>
  );
};

const getPrimaryKeys = (program: Program, model: Model) =>
  pipe(
    getModelProperties(program, model),
    HashMap.toValues,
    Array.filterMap(
      flow(
        Option.liftPredicate((_) => isKey(program, _) || normalKeys(program).has(_)),
        Option.map((_) => _.name),
      ),
    ),
  );

interface EntityProps {
  model: Model;
}

const Entity = ({ model }: EntityProps) => {
  const { program } = useTsp();

  const baseName = pipe(entities(program).get(model)!, (_) => getModelName(program, _));
  const key = `${useContext(SourceDirectoryContext)?.path}/${baseName}`;

  const primaryKeys = getPrimaryKeys(program, model);

  const requiredKeys = pipe(
    getModelProperties(program, model),
    HashMap.toValues,
    Array.filterMap(
      flow(
        Option.liftPredicate((_) => !_.optional),
        Option.map((_) => _.name),
      ),
    ),
  );

  const refkeys = refkey('schema', getModelRefKey(program, model));

  return (
    <ClassDeclaration
      export
      extends={
        <FunctionCallExpression
          args={[
            <ObjectExpression>
              <CommaList>
                <ObjectProperty name='message' value={refkeys} />
                <ObjectProperty jsValue={key} name='key' />
                <ObjectProperty jsValue={primaryKeys} name='primaryKeys' />
                <ObjectProperty jsValue={requiredKeys} name='requiredKeys' />
                {pipe(
                  schemaOutput({ origin: true, program, type: model }),
                  Option.map((_) => <ObjectProperty name='schema' value={_} />),
                  Option.getOrNull,
                )}
              </CommaList>
            </ObjectExpression>,
          ]}
          target={refkey('makeEntity')}
        />
      }
      name={`${getModelName(program, model)}Entity`}
      refkey={refkey(model)}
    />
  );
};

interface EndpointProps {
  meta: EndpointMeta;
  operation: Operation;
}

const Endpoint = ({ meta, operation }: EndpointProps) => {
  const { $, program } = useTsp();

  const options = pipe(
    meta.options?.properties.entries().toArray() ?? [],
    Array.filterMap(([key, { type }]) =>
      Option.gen(function* () {
        const template = yield* Option.liftPredicate(type, (_) => $.model.is(_));

        const target = yield* pipe(
          template.templateMapper?.args[0],
          Option.fromNullable,
          Option.filter((_) => $.type.is(_)),
        );

        if (template.name === 'Schema') {
          const schema = yield* schemaOutput({ program, type: target });
          return <ObjectProperty name={key} value={schema} />;
        }

        if (template.name === 'PrimaryKeys' && $.model.is(target)) {
          const primaryKeys = getPrimaryKeys(program, target);
          return <ObjectProperty jsValue={primaryKeys} name={key} />;
        }
      }),
    ),
  );

  const name = pipe(
    [useContext(SourceDirectoryContext)?.path, operation.interface?.name, operation.name],
    Array.filter(Predicate.isNotNullable),
    Array.join('/'),
  );

  return (
    <VarDeclaration const export name={`${operation.name}Endpoint`}>
      <FunctionCallExpression
        args={[
          <ObjectExpression>
            <CommaList>
              {[
                <ObjectProperty
                  name='method'
                  value={
                    <>
                      {refkey('service', operation.interface)}.method.{String.uncapitalize(operation.name)}
                    </>
                  }
                />,
                <ObjectProperty jsValue={name} name='name' />,
                ...options,
              ]}
            </CommaList>
          </ObjectExpression>,
        ]}
        target={refkey('method', meta.method)}
      />
    </VarDeclaration>
  );
};

interface SchemaOutputProps {
  origin?: boolean;
  program: Program;
  type: Type;
}

const schemaOutput = ({ origin, program, type }: SchemaOutputProps): Option.Option<Children> => {
  if (!$(program).model.is(type)) return Option.none();

  if ($(program).array.is(type)) {
    const element = $(program).array.getElementType(type);
    return Option.map(schemaOutput({ program, type: element }), (_) => <ArrayExpression>{_}</ArrayExpression>);
  }

  if (!origin && entities(program).has(type)) return Option.some(refkey(type));

  return pipe(
    getModelProperties(program, type),
    HashMap.toValues,
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
