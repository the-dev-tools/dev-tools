import {
  createTypeSpecLibrary,
  DecoratorContext,
  EnumMember,
  Model,
  ModelProperty,
  Operation,
  Scalar,
  Type,
  UnionVariant,
} from '@typespec/compiler';
import { pipe, Schema } from 'effect';
import { makeEmitterOptions, makeStateFactory } from '../utils.js';

export class EmitterOptions extends Schema.Class<EmitterOptions>('EmitterOptions')({
  goPackage: pipe(Schema.String, Schema.optionalWith({ as: 'Option' })),
}) {}

export const $lib = createTypeSpecLibrary({
  diagnostics: {},
  emitter: { options: makeEmitterOptions(EmitterOptions) },
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

const { makeStateMap } = makeStateFactory((_) => $lib.createStateSymbol(_));

export const streams = makeStateMap<Operation, 'Duplex' | 'In' | 'None' | 'Out'>('streams');
export const externals = makeStateMap<Type, [string, string]>('externals');
export const maps = makeStateMap<Type, [Type, Type]>('maps');
export const optionMap = makeStateMap<Type, [string, unknown][]>('options');
export const fieldNumberMap = makeStateMap<ModelProperty | UnionVariant, number>('fieldNumber');

function stream({ program }: DecoratorContext, target: Operation, mode: EnumMember) {
  streams(program).set(target, mode.name as never);
}

function external({ program }: DecoratorContext, target: Model, path: string, name: string) {
  if (target.sourceModel === undefined) return;
  externals(program).set(target, [path, name]);
}

function _map({ program }: DecoratorContext, target: Scalar, key: Type, value: Type) {
  maps(program).set(target, [key, value]);
}
