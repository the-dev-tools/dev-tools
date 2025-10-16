import {
  BasicScope,
  BasicSymbol,
  Binder,
  Block,
  Children,
  createContext,
  Declaration,
  For,
  getSymbolCreatorSymbol,
  List,
  memo,
  Name,
  OutputScope,
  OutputScopeOptions,
  OutputSymbol,
  OutputSymbolOptions,
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
import {
  EmitContext,
  Enum,
  Interface,
  isNullType,
  isTemplateDeclaration,
  Model,
  ModelProperty,
  Namespace,
  Program,
  Type,
} from '@typespec/compiler';
import { Output, useTsp, writeOutput } from '@typespec/emitter-framework';
import {
  Array,
  flow,
  Hash,
  HashMap,
  Match,
  Number,
  Option,
  pipe,
  Predicate,
  Record,
  Schema,
  String,
  Tuple,
} from 'effect';
import { join } from 'node:path/posix';
import { Projects, useProject } from '../core/index.js';
import { EmitterOptions, externals, maps, optionMap, streams } from './lib.js';

const EmitterOptionsContext = createContext<EmitterOptions>();

export const $onEmit = async (context: EmitContext<(typeof EmitterOptions)['Encoded']>) => {
  const { emitterOutputDir, program } = context;

  const options = Schema.decodeSync(EmitterOptions)(context.options);

  if (program.compilerOptions.noEmit) return;

  const globalScope = new BasicScope('global', undefined);

  const bindExternals = (binder: Binder) => {
    const scopeMap = new Map<string, ExternalScope>();

    externals(program).forEach(([path, name], type) => {
      const scope = scopeMap.get(path) ?? new ExternalScope(path, globalScope, { binder });
      scopeMap.set(path, scope);
      new BasicSymbol(name, scope.spaces, { binder, refkeys: refkey(type) });
    });
  };

  await writeOutput(
    program,
    <EmitterOptionsContext.Provider value={options}>
      <Scope value={globalScope}>
        <Output externals={[{ [getSymbolCreatorSymbol()]: bindExternals }]} printWidth={120} program={program}>
          <Projects>
            {(_) =>
              pipe(
                _.namespace.namespaces.values(),
                Array.fromIterable,
                Array.map((_) => <Package namespace={_} />),
              )
            }
          </Projects>
        </Output>
      </Scope>
    </EmitterOptionsContext.Provider>,
    join(emitterOutputDir, 'protobuf'),
  );
};

// https://protobuf.dev/programming-guides/proto3/#assigning
const fieldNumberFromName = (value: string) => {
  const fieldNumber = Math.abs(Hash.string(value) % 536_870_911);
  if (Number.between(fieldNumber, { maximum: 19_999, minimum: 19_000 })) return Math.trunc(fieldNumber / 10);
  return fieldNumber;
};

// https://protobuf.dev/programming-guides/proto3/#enum
// const enumNumberFromName = (value: string) => Math.abs(Hash.string(value) % 2 ** 32);

// https://protobuf.dev/programming-guides/proto3/#scalar
const protoScalarsMapCache = new WeakMap<Program, HashMap.HashMap<Type, string>>();
const useProtoScalarsMap = () => {
  const { program } = useTsp();

  let scalarMap = protoScalarsMapCache.get(program);
  if (scalarMap) return scalarMap;

  scalarMap = pipe(
    [
      ['DevTools.Protobuf.fixed32', 'fixed32'],
      ['DevTools.Protobuf.fixed64', 'fixed64'],
      ['DevTools.Protobuf.sfixed32', 'sfixed32'],
      ['DevTools.Protobuf.sfixed64', 'sfixed64'],
      ['DevTools.Protobuf.sint32', 'sint32'],
      ['DevTools.Protobuf.sint64', 'sint64'],
      ['TypeSpec.boolean', 'bool'],
      ['TypeSpec.bytes', 'bytes'],
      ['TypeSpec.float32', 'float'],
      ['TypeSpec.float64', 'double'],
      ['TypeSpec.int32', 'int32'],
      ['TypeSpec.int64', 'int64'],
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
        (_) => $.model.is(_),
        (_) => Option.some(refkey(_)),
      ),
      Match.when(
        (_) => $.enum.is(_),
        (_) => Option.some(refkey(_)),
      ),
      Match.when(isNullType, (_) =>
        pipe(
          program.resolveTypeReference('DevTools.Protobuf.WellKnown.Null')[0],
          Option.fromNullable,
          Option.map(refkey),
        ),
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

interface TypeReferenceProps {
  path: string;
}

const TypeReference = ({ path }: TypeReferenceProps) => {
  const { program } = useTsp();
  return <>{refkey(program.resolveTypeReference(path)[0])}</>;
};

class ExternalScope extends BasicScope {
  kind = 'external' as const;
}

interface PackageScopeOptions extends OutputScopeOptions {
  specifier: string;
}

class PackageScope extends BasicScope {
  kind = 'package' as const;

  imports = reactive(new Set()) as Set<ExternalScope | PackageScope>;
  specifier: string;

  constructor(name: string, parentScope: OutputScope | undefined, options: PackageScopeOptions) {
    super(name, parentScope, options);
    this.specifier = options.specifier;
  }
}

interface BasicDeclarationProps extends OutputSymbolOptions {
  children: Children;
  name: string;
}

const BasicDeclaration = ({ children, name, ...props }: BasicDeclarationProps) => {
  const scope = useScope();
  const symbol = new BasicSymbol(name, scope.spaces, props);
  return <Declaration symbol={symbol}>{children}</Declaration>;
};

interface PackageProps {
  namespace: Namespace;
}

const Package = ({ namespace }: PackageProps) => {
  const { $ } = useTsp();
  const { goPackage } = useContext(EmitterOptionsContext)!;
  const { version } = useProject();

  const name = String.pascalToSnake(namespace.name);

  const parent = useContext(SourceDirectoryContext)?.path;

  let path = `${name}/v${version}`;
  if (parent && parent !== './') path = `${parent}/${path}`;

  const specifier = path.replaceAll('/', '.');

  const parentScope = useScope();
  const scope = new PackageScope(`${path}/${name}.proto`, parentScope, { specifier });

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

  const messages = pipe(namespace.models.values().toArray(), (_) => (
    <Show when={_.length > 0}>
      <hbr />
      <For doubleHardline each={_}>
        {(_) => <Message model={_} />}
      </For>
      <hbr />
    </Show>
  ));

  const services = pipe(
    namespace.interfaces.values(),
    Array.fromIterable,
    Array.filter((_) => {
      if (!_.isFinished) $.type.finishType(_);
      return !isTemplateDeclaration(_);
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
        <SourceFile filetype='string' path={`${name}.proto`} reference={PackageReference}>
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

interface PackageReferenceProps {
  refkey: Refkey;
}

const PackageReference = ({ refkey }: PackageReferenceProps) => {
  const resolveResult: Ref<ResolutionResult<ExternalScope | PackageScope, OutputSymbol> | undefined> = resolve(refkey);
  const scope = useScope() as PackageScope;

  return memo(() =>
    pipe(
      Option.gen(function* () {
        const result = yield* Option.fromNullable(resolveResult.value);

        if (scope === result.commonScope) return result.lexicalDeclaration.name;

        const targetScope = yield* Array.head(result.pathDown);

        scope.imports.add(targetScope);

        const targetName = result.lexicalDeclaration.name;
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
        {(_, index) => {
          const name = fieldName(_.name);
          // TODO: use `enumNumberFromName(name)` instead of `index + 1` once server enum usage is fixed
          return `${name} = ${_.value ?? index + 1}`;
        }}
      </For>
    </Block>
  );

  return (
    <BasicDeclaration name={_enum.name} refkeys={refkey(_enum)}>
      enum <Name /> {fields}
    </BasicDeclaration>
  );
};

interface MessageProps {
  model: Model;
}

const Message = ({ model }: MessageProps) => {
  const { $, program } = useTsp();

  const options = pipe(
    optionMap(program).get(model) ?? [],
    Option.liftPredicate(Array.isNonEmptyArray),
    Option.map((_) => (
      <For each={_}>
        {(_) => (
          <>
            option <OptionValue>{_}</OptionValue>;
          </>
        )}
      </For>
    )),
    Option.getOrNull,
  );

  const fields = pipe(
    $.model.getProperties(model).values(),
    Array.fromIterable,
    Option.liftPredicate(Array.isNonEmptyArray),
    Option.map((_) => <For each={_}>{(_) => <Field property={_} />}</For>),
    Option.getOrNull,
  );

  return (
    <BasicDeclaration name={model.name} refkeys={refkey(model)}>
      message <Name />{' '}
      <Block>
        <List doubleHardline>{...[options, fields]}</List>
      </Block>
    </BasicDeclaration>
  );
};

interface FieldProps {
  property: ModelProperty;
}

const Field = ({ property }: FieldProps) => {
  const { $, program } = useTsp();
  const protoTypeMap = useProtoTypeMap();

  const type = pipe(property.type, protoTypeMap, Option.getOrThrow);
  const number = fieldNumber(program).get(property) ?? fieldNumberFromName(property.name);

  const repeatedOrOptional = $.array.is(property.type) ? 'repeated' : property.optional && 'optional';

  const options = optionMap(program).get(property) ?? [];

  if ($.model.is(property.type) && !$.array.is(property.type) && !property.optional)
    options.push(['DevTools.Protobuf.Validate.Field.Required', true]);

  return (
    <>
      <List space>
        {[
          repeatedOrOptional,
          type,
          `${String.camelToSnake(property.name)} = ${number}`,
          options.length > 0 && (
            <Block closer=']' inline opener='['>
              <For comma each={options} line>
                {(_) => <OptionValue>{_}</OptionValue>}
              </For>
            </Block>
          ),
        ]}
      </List>
      ;
    </>
  );
};

interface OptionValueProps {
  children: [string, unknown];
}

const OptionValue = ({ children: [reference, value] }: OptionValueProps) => (
  <>
    <TypeReference path={reference} /> = <Value>{value}</Value>
  </>
);

class ValueLiteral {
  constructor(public value: string) {}
}

interface ValueProps {
  children: unknown;
}

const Value = ({ children }: ValueProps) =>
  pipe(
    Match.value(children),
    Match.when(Predicate.isString, (_) => `"${_}"`),
    Match.when(Predicate.isNumber, (_) => _.toString()),
    Match.when(Predicate.isBoolean, (_) => _.toString()),
    Match.when(
      (_: unknown) => _ instanceof ValueLiteral,
      (_) => _.value,
    ),
    Match.when(Predicate.isIterable, (_) => (
      <Block closer=']' inline opener='['>
        <For comma each={Array.fromIterable(_)} line>
          {(_) => <Value>{_}</Value>}
        </For>
      </Block>
    )),
    Match.when(Predicate.isRecord, (_) => (
      <Block inline>
        <For comma each={Record.toEntries(_)} line>
          {([key, _]) => (
            <>
              {key}: <Value>{_}</Value>
            </>
          )}
        </For>
      </Block>
    )),
    Match.orElse(() => null),
  );

interface ServiceProps {
  _interface: Interface;
}

const Service = ({ _interface }: ServiceProps) => {
  const { $, program } = useTsp();
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

              const empty = Option.fromNullable(program.resolveTypeReference('DevTools.Protobuf.WellKnown.Empty')[0]);

              const inputType = yield* pipe(
                _.parameters.sourceModels,
                Option.liftPredicate(Array.isNonEmptyArray),
                Option.match({
                  onNone: () => empty,
                  onSome: flow(
                    Array.findFirst((_) => _.usage === 'spread'),
                    Option.map((_) => _.model),
                  ),
                }),
                Option.flatMap(protoTypeMap),
              );

              const outputType = yield* pipe(
                _.returnType,
                Option.liftPredicate((_) => $.model.is(_)),
                Option.filter((_) => _.name.length > 0),
                Option.orElse(() => empty),
                Option.flatMap(protoTypeMap),
              );

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
    <BasicDeclaration name={_interface.name} refkeys={refkey(_interface)}>
      service <Name /> {fields}
    </BasicDeclaration>
  );
};
