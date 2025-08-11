import {
  Binder,
  Block,
  Children,
  createContext,
  Declaration,
  For,
  getSymbolCreatorSymbol,
  memo,
  Name,
  OutputScope,
  OutputScopeOptions,
  OutputSymbol,
  reactive,
  Ref,
  Refkey,
  refkey,
  ResolutionResult,
  resolve,
  Scope,
  Show,
  SourceDirectory,
  SourceDirectoryContext,
  SourceFile,
  useContext,
  useScope,
} from '@alloy-js/core';
import { EmitContext, Enum, Interface, Model, ModelProperty, Namespace, Program, Type } from '@typespec/compiler';
import { Output, useTsp, writeOutput } from '@typespec/emitter-framework';
import { Array, Hash, HashMap, Match, Number, Option, pipe, Schema, String, Tuple } from 'effect';
import { EmitterOptions } from './lib.js';
import { externals, maps, streams } from './state.js';

const EmitterOptionsContext = createContext<EmitterOptions>();

export const $onEmit = async (context: EmitContext<(typeof EmitterOptions)['Encoded']>) => {
  const { emitterOutputDir, program } = context;

  const options = Schema.decodeSync(EmitterOptions)(context.options);

  if (program.compilerOptions.noEmit) return;

  const root = program.getGlobalNamespaceType().namespaces.get(options.rootNamespace);
  if (!root) return;

  const bindExternals = (binder: Binder) => {
    const scopeMap = new Map<string, ExternalScope>();

    externals(program).forEach(([path, name], type) => {
      let scope = scopeMap.get(path);
      if (!scope) {
        scope = new ExternalScope(path, { parent: binder.globalScope });
        scopeMap.set(path, scope);
      }

      const symbol = new OutputSymbol(name, { refkeys: refkey(type), scope });

      binder.notifyScopeCreated(scope);
      binder.notifySymbolCreated(symbol);
    });
  };

  await writeOutput(
    program,
    <EmitterOptionsContext.Provider value={options}>
      <Output externals={[{ [getSymbolCreatorSymbol()]: bindExternals }]} program={program}>
        {pipe(
          root.namespaces.values(),
          Array.fromIterable,
          Array.map((_) => <Package namespace={_} />),
        )}
      </Output>
    </EmitterOptionsContext.Provider>,
    emitterOutputDir,
  );
};

// https://protobuf.dev/programming-guides/proto3/#assigning
const fieldNumberFromName = (value: string) => {
  const fieldNumber = Math.abs(Hash.string(value) % 536_870_911);
  if (Number.between(fieldNumber, { maximum: 19_999, minimum: 19_000 })) return Math.trunc(fieldNumber / 10);
  return fieldNumber;
};

// https://protobuf.dev/programming-guides/proto3/#scalar
const protoScalarsMapCache = new WeakMap<Program, HashMap.HashMap<Type, string>>();
function useProtoScalarsMap() {
  const { program } = useTsp();

  let scalarMap = protoScalarsMapCache.get(program);
  if (scalarMap) return scalarMap;

  scalarMap = pipe(
    [
      ['TypeSpec.boolean', 'bool'],
      ['TypeSpec.bytes', 'bytes'],
      ['TypeSpec.fixed32', 'fixed32'],
      ['TypeSpec.fixed64', 'fixed64'],
      ['TypeSpec.float32', 'float'],
      ['TypeSpec.float64', 'double'],
      ['TypeSpec.int32', 'int32'],
      ['TypeSpec.int64', 'int64'],
      ['TypeSpec.sfixed32', 'sfixed32'],
      ['TypeSpec.sfixed64', 'sfixed64'],
      ['TypeSpec.sint32', 'sint32'],
      ['TypeSpec.sint64', 'sint64'],
      ['TypeSpec.string', 'string'],
      ['TypeSpec.uint32', 'uint32'],
      ['TypeSpec.uint64', 'uint64'],
    ] as const,
    Array.filterMap(([ref, scalar]) =>
      pipe(
        program.resolveTypeReference(ref),
        Tuple.getFirst,
        Option.fromNullable,
        Option.map((_: Type) => [_, scalar] as const),
      ),
    ),
    HashMap.fromIterable,
  );

  protoScalarsMapCache.set(program, scalarMap);
  return scalarMap;
}

class ExternalScope extends OutputScope {
  override get kind() {
    return 'external' as const;
  }
}

interface PackageScopeOptions extends OutputScopeOptions {
  specifier: string;
}

class PackageScope extends OutputScope {
  imports = reactive(new Set()) as Set<ExternalScope | PackageScope>;
  specifier: string;

  override get kind() {
    return 'package' as const;
  }

  constructor(name: string, options: PackageScopeOptions) {
    super(name, options);
    this.specifier = options.specifier;
  }
}

interface PackageProps {
  namespace: Namespace;
}

const Package = ({ namespace }: PackageProps) => {
  const { goPackage, version } = useContext(EmitterOptionsContext)!;

  const name = String.pascalToSnake(namespace.name);

  const parent = useContext(SourceDirectoryContext)?.path;

  let path = `${name}/v1`;
  if (parent && parent !== './') path = `${parent}/${path}`;

  const specifier = path.replaceAll('/', '.');

  const headers = ['syntax = "proto3"', `package ${specifier}`];
  if (Option.isSome(goPackage)) headers.push(`option go_package = "${goPackage.value}/${path};${name}v${version}"`);

  const header = (
    <For doubleHardline each={headers} enderPunctuation semicolon>
      {(_) => _}
    </For>
  );

  const packages = pipe(
    namespace.namespaces.values(),
    Array.fromIterable,
    Array.map((_) => <Package namespace={_} />),
  );

  const enums = (
    <Show when={namespace.enums.size > 0}>
      <hbr />
      <For doubleHardline each={namespace.enums.values()}>
        {(_) => <ProtoEnum _enum={_} />}
      </For>
      <hbr />
    </Show>
  );

  const messages = (
    <Show when={namespace.models.size > 0}>
      <hbr />
      <For doubleHardline each={namespace.models.values()}>
        {(_) => <Message model={_} />}
      </For>
      <hbr />
    </Show>
  );

  const scope = new PackageScope(`${path}/${name}.proto`, { specifier });

  const imports = (
    <Show when={scope.imports.size > 0}>
      <hbr />
      <For each={scope.imports.values()} enderPunctuation hardline semicolon>
        {(_) => `import "${_.name}"`}
      </For>
      <hbr />
    </Show>
  );

  const services = (
    <Show when={namespace.interfaces.size > 0}>
      <hbr />
      <For doubleHardline each={namespace.interfaces.values()}>
        {(_) => <Service _interface={_} />}
      </For>
      <hbr />
    </Show>
  );

  return (
    <SourceDirectory path={name}>
      {packages}

      <SourceDirectory path={`v${version}`}>
        <SourceFile filetype='string' path={`${name}.proto`} reference={Reference}>
          <Scope value={scope}>
            {header}
            <hbr />
            {imports}
            {enums}
            {messages}
            {services}
          </Scope>
        </SourceFile>
      </SourceDirectory>
    </SourceDirectory>
  );
};

interface ReferenceProps {
  refkey: Refkey;
}

const Reference = ({ refkey }: ReferenceProps) => {
  const resolveResult: Ref<ResolutionResult<ExternalScope | PackageScope, OutputSymbol> | undefined> = resolve(refkey);
  const scope = useScope() as PackageScope;

  return memo(() =>
    pipe(
      Option.gen(function* () {
        const result = yield* Option.fromNullable(resolveResult.value);

        if (scope === result.commonScope) return result.targetDeclaration.name;

        const targetScope = yield* Array.head(result.pathDown);

        scope.imports.add(targetScope);

        const targetName = result.targetDeclaration.name;
        if (targetScope.kind === 'external') return targetName;

        const packageName = targetScope.specifier;
        return `${packageName}.${targetName}`;
      }),
      Option.getOrElse(() => 'UNRESOLVED_SYMBOL'),
    ),
  );
};

interface ProtoEnumProps {
  _enum: Enum;
}

const ProtoEnum = ({ _enum }: ProtoEnumProps) => {
  const fields = pipe(
    _enum.members.values(),
    Array.fromIterable,
    Option.liftPredicate(Array.isNonEmptyArray),
    Option.map((_) => (
      <Block>
        <For each={_} enderPunctuation hardline semicolon>
          {(_) => `${pipe(_.name, String.pascalToSnake, String.toUpperCase)} = ${_.value}`}
        </For>
      </Block>
    )),
    Option.getOrElse(() => '{}'),
  );

  return (
    <Declaration name={_enum.name} refkey={refkey(_enum)}>
      enum <Name /> {fields}
    </Declaration>
  );
};

interface MessageProps {
  model: Model;
}

const Message = ({ model }: MessageProps) => {
  const fields = pipe(
    model.properties.values(),
    Array.fromIterable,
    Option.liftPredicate(Array.isNonEmptyArray),
    Option.map((_) => (
      <Block>
        <For each={_}>{(_) => <Field property={_} />}</For>
      </Block>
    )),
    Option.getOrElse(() => '{}'),
  );

  return (
    <Declaration name={model.name} refkey={refkey(model)}>
      message <Name /> {fields}
    </Declaration>
  );
};

interface FieldProps {
  property: ModelProperty;
}

const Field = ({ property }: FieldProps) => {
  const { $, program } = useTsp();

  const protoScalarsMap = useProtoScalarsMap();

  const getMapProtoType = (type: Type) => {
    const types = pipe(maps(program).get(type)!, Tuple.map(getProtoType), Array.getSomes);
    if (!Tuple.isTupleOf(types, 2)) throw Error('Incorrect map');
    const [key, value] = types;

    return Option.some(
      <>
        map {'<'} {key}, {value} {'>'}
      </>,
    );
  };

  const getProtoType = (type: Type): Option.Option<Children> =>
    pipe(
      Match.value(type),
      Match.when(
        (_) => $.array.is(_),
        (_) => pipe($.array.getElementType(_), getProtoType),
      ),
      Match.when(
        (_) => maps(program).has(_),
        (_) => getMapProtoType(_),
      ),
      Match.when(
        (_) => $.model.is(_) || $.enum.is(_),
        (_) => Option.some(refkey(_)),
      ),
      Match.when(
        (_) => $.scalar.is(_),
        (_) => HashMap.get(protoScalarsMap, _),
      ),
      Match.option,
      Option.flatten,
    );

  const type = pipe(property.type, getProtoType, Option.getOrThrow);

  const number = fieldNumberFromName(property.name);

  return (
    <>
      {$.array.is(property.type) ? 'repeated ' : property.optional && 'optional '}
      {type} {property.name} = {number};
    </>
  );
};

interface ServiceProps {
  _interface: Interface;
}

const Service = ({ _interface }: ServiceProps) => {
  const { program } = useTsp();

  const fields = pipe(
    _interface.operations.values(),
    Array.fromIterable,
    Option.liftPredicate(Array.isNonEmptyArray),
    Option.map((_) => (
      <Block>
        <For each={_} enderPunctuation hardline semicolon>
          {(_) => {
            const streamKey = 'stream ';
            const [inputStreamKey, outputStreamKey] = pipe(
              streams(program).get(_) ?? 'None',
              Match.value,
              Match.when('None', () => ['', ''] as const),
              Match.when('Duplex', () => [streamKey, streamKey] as const),
              Match.when('In', () => [streamKey, ''] as const),
              Match.when('Out', () => ['', streamKey] as const),
              Match.exhaustive,
            );

            const { model: inputType } = pipe(
              _.parameters.sourceModels,
              Array.findFirst((_) => _.usage === 'spread'),
              Option.getOrThrow,
            );

            return (
              <>
                rpc {_.name}({inputStreamKey}
                {refkey(inputType)}) returns ({outputStreamKey}
                {refkey(_.returnType)})
              </>
            );
          }}
        </For>
      </Block>
    )),
    Option.getOrElse(() => '{}'),
  );

  return (
    <Declaration name={_interface.name} refkey={refkey(_interface)}>
      service <Name /> {fields}
    </Declaration>
  );
};
