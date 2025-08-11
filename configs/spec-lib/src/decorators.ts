import { DecoratorContext, EnumMember, Model, Operation, Scalar, StringLiteral, Type } from '@typespec/compiler';
import { externals, maps, streams } from './state.js';

const external = ({ program }: DecoratorContext, target: Model, path: StringLiteral, name: StringLiteral) =>
  externals(program).set(target, [path.value, name.value]);

const _map = ({ program }: DecoratorContext, target: Scalar, key: Type, value: Type) =>
  maps(program).set(target, [key, value]);

const stream = ({ program }: DecoratorContext, target: Operation, mode: EnumMember) =>
  streams(program).set(target, mode.name as never);

export const $decorators = {
  TypeSpec: {
    stream,
  },
  'TypeSpec.Private': {
    external,
    map: _map,
  },
};
