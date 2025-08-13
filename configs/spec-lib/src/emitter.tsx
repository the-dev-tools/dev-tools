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
import { Array, flow, Hash, HashMap, Match, Number, Option, pipe, Schema, String, Tuple } from 'effect';
import { EmitterOptions } from './lib.js';
import { externals, instances, instancesByModel, instancesByTemplate, maps, streams, templates } from './state.js';

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

// https://protobuf.dev/programming-guides/proto3/#enum
const enumNumberFromName = (value: string) => Math.abs(Hash.string(value) % 2 ** 32);

// https://protobuf.dev/programming-guides/proto3/#scalar
const protoScalarsMapCache = new WeakMap<Program, HashMap.HashMap<Type, string>>();
const useProtoScalarsMap = () => {
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
};

const useProtoTypeMap = () => {
  const { $, program } = useTsp();

  const protoScalarsMap = useProtoScalarsMap();

  const getProtoType = (type: Type): Option.Option<Children> =>
    pipe(
      Match.value(type),
      Match.when(
        (_) => $.array.is(_),
        (_) => pipe($.array.getElementType(_), getProtoType),
      ),
      Match.when(
        (_) => maps(program).has(_),
        (_) =>
          pipe(
            maps(program).get(_),
            Option.fromNullable,
            Option.flatMap(flow(Tuple.map(getProtoType), Array.getSomes, Option.liftPredicate(Tuple.isTupleOf(2)))),
            Option.map(([key, value]) => ['map <', key, ', ', value, '>']),
          ),
      ),
      Match.when(
        (_) => instancesByTemplate(program).has(_),
        (_) =>
          pipe(
            Option.liftPredicate(_, (_) => $.model.is(_)),
            Option.flatMapNullable((_) => instancesByTemplate(program).get(_)),
            Option.flatMap(getProtoType),
          ),
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

  return getProtoType;
};

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
  const { $, program } = useTsp();
  const { goPackage, version } = useContext(EmitterOptionsContext)!;

  const name = String.pascalToSnake(namespace.name);

  const parent = useContext(SourceDirectoryContext)?.path;

  let path = `${name}/v1`;
  if (parent && parent !== './') path = `${parent}/${path}`;

  const specifier = path.replaceAll('/', '.');

  const scope = new PackageScope(`${path}/${name}.proto`, { specifier });

  const packages = pipe(
    namespace.namespaces.values(),
    Array.fromIterable,
    Array.map((_) => <Package namespace={_} />),
  );

  const headers = ['syntax = "proto3"', `package ${specifier}`];
  if (Option.isSome(goPackage)) headers.push(`option go_package = "${goPackage.value}/${path};${name}v${version}"`);

  const header = (
    <For doubleHardline each={headers} enderPunctuation semicolon>
      {(_) => _}
    </For>
  );

  const imports = (
    <Show when={scope.imports.size > 0}>
      <hbr />
      <For each={scope.imports.values()} enderPunctuation hardline semicolon>
        {(_) => `import "${_.name}"`}
      </For>
      <hbr />
    </Show>
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

  const messages = pipe(
    namespace.models.values(),
    Array.fromIterable,
    Array.filterMap((_) => {
      if (!_.isFinished) $.type.finishType(_);
      if (templates(program).has(_)) return Option.none();
      if (instances(program).has(_)) return Option.none();
      return pipe(instancesByModel(program).get(_)?.values() ?? [], Array.fromIterable, Array.prepend(_), Option.some);
    }),
    Array.flatten,
    (_) => (
      <Show when={_.length > 0}>
        <hbr />
        <For doubleHardline each={_}>
          {(_) => <Message model={_} />}
        </For>
        <hbr />
      </Show>
    ),
  );

  const services = pipe(
    namespace.interfaces.values(),
    Array.fromIterable,
    Array.filter((_) => {
      if (!_.isFinished) $.type.finishType(_);
      return !templates(program).has(_);
    }),
    (_) => (
      <Show when={_.length > 0}>
        <hbr />
        <For doubleHardline each={_}>
          {(_) => <Service _interface={_} />}
        </For>
        <hbr />
      </Show>
    ),
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
  const fieldName = (_: string) => pipe(_enum.name + _, String.pascalToSnake, String.toUpperCase);

  const fields = (
    <Block>
      {fieldName('Unspecified')} = 0;
      <hbr />
      <For each={_enum.members.values()} enderPunctuation hardline semicolon>
        {(_) => {
          const name = fieldName(_.name);
          return `${name} = ${_.value ?? enumNumberFromName(name)}`;
        }}
      </For>
    </Block>
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
  const { $ } = useTsp();
  const protoTypeMap = useProtoTypeMap();

  const type = pipe(property.type, protoTypeMap, Option.getOrThrow);
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
  const protoTypeMap = useProtoTypeMap();

  const fields = pipe(
    _interface.operations.values(),
    Array.fromIterable,
    Option.liftPredicate(Array.isNonEmptyArray),
    Option.map((_) => (
      <Block>
        <For each={_} enderPunctuation hardline semicolon>
          {(_) =>
            Option.gen(function* () {
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

              const inputType = yield* pipe(
                _.parameters.sourceModels,
                Option.liftPredicate(Array.isNonEmptyArray),
                Option.match({
                  onNone: () => Option.fromNullable(program.resolveTypeReference('TypeSpec.WellKnown.Empty')[0]),
                  onSome: flow(
                    Array.findFirst((_) => _.usage === 'spread'),
                    Option.map((_) => _.model),
                  ),
                }),
                Option.flatMap(protoTypeMap),
              );

              const outputType = yield* protoTypeMap(_.returnType);

              return (
                <>
                  rpc {_.name}({inputStreamKey}
                  {refkey(inputType)}) returns ({outputStreamKey}
                  {refkey(outputType)})
                </>
              );
            }).pipe(Option.getOrThrow)
          }
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
