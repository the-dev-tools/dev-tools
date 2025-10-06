import {
  createTypeSpecLibrary,
  DecoratorContext,
  JSONSchemaType,
  Model,
  Operation,
  StringLiteral,
} from '@typespec/compiler';
import { JSONSchema } from 'effect';
import { EmitterOptions } from '../core/index.js';
import { makeStateFactory } from '../utils.js';

export const $lib = createTypeSpecLibrary({
  diagnostics: {},
  emitter: { options: JSONSchema.make(EmitterOptions) as JSONSchemaType<EmitterOptions> },
  name: '@the-dev-tools/spec-lib/data-client',
});

export const $decorators = {
  'DevTools.DataClient': {
    endpoint,
    entity,
  },
};

const { makeStateMap } = makeStateFactory((_) => $lib.createStateSymbol(_));

export interface EndpointMeta {
  method: string;
  options: Model | undefined;
}

export const entities = makeStateMap<Model, Model>('entities');
export const endpoints = makeStateMap<Operation, EndpointMeta>('endpoints');

function entity({ program }: DecoratorContext, target: Model, base?: Model) {
  entities(program).set(target, base ?? target);
}

function endpoint({ program }: DecoratorContext, target: Operation, method: StringLiteral, options?: Model) {
  endpoints(program).set(target, { method: method.value, options });
}
