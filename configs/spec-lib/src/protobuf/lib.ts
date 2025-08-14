import {
  createTypeSpecLibrary,
  DecoratorContext,
  EnumMember,
  JSONSchemaType,
  Model,
  Operation,
  Program,
  Scalar,
  StringLiteral,
  Type,
} from '@typespec/compiler';
import { JSONSchema, pipe, Record, Schema } from 'effect';

export class EmitterOptions extends Schema.Class<EmitterOptions>('EmitterOptions')({
  goPackage: pipe(Schema.String, Schema.optionalWith({ as: 'Option' })),
  rootNamespace: pipe(Schema.String, Schema.optionalWith({ default: () => 'API' })),
  version: pipe(Schema.Positive, Schema.int(), Schema.optionalWith({ default: () => 1 })),
}) {}

export const $lib = createTypeSpecLibrary({
  diagnostics: {},
  emitter: { options: pipe(EmitterOptions.fields, Schema.Struct, JSONSchema.make) as JSONSchemaType<EmitterOptions> },
  name: '@the-dev-tools/spec-lib/protobuf',
});

export const $decorators = {
  'DevTools.Protobuf': {
    stream,
  },
  'DevTools.Protobuf.Private': {
    external,
    map: _map,
  },
};

const stateKeys = ['streams', 'externals', 'maps'] as const;
export const state: Record<(typeof stateKeys)[number], symbol> = Record.fromIterableWith(stateKeys, (_) => [
  _,
  $lib.createStateSymbol(_),
]);

export const streams = (program: Program) =>
  program.stateMap(state.streams) as Map<Operation, 'Duplex' | 'In' | 'None' | 'Out'>;

export const externals = (program: Program) => program.stateMap(state.externals) as Map<Type, [string, string]>;

export const maps = (program: Program) => program.stateMap(state.maps) as Map<Type, [Type, Type]>;

function stream({ program }: DecoratorContext, target: Operation, mode: EnumMember) {
  streams(program).set(target, mode.name as never);
}

function external({ program }: DecoratorContext, target: Model, path: StringLiteral, name: StringLiteral) {
  if (target.sourceModel === undefined) return;
  externals(program).set(target, [path.value, name.value]);
}

function _map({ program }: DecoratorContext, target: Scalar, key: Type, value: Type) {
  maps(program).set(target, [key, value]);
}
