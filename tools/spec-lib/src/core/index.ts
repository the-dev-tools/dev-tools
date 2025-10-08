import { createTypeSpecLibrary, DecoratorContext, isKey, Model, ModelProperty, Program } from '@typespec/compiler';
import { $ } from '@typespec/compiler/typekit';
import { Array, Option, pipe, Schema } from 'effect';
import { makeStateFactory } from '../utils.js';

export const $lib = createTypeSpecLibrary({
  diagnostics: {},
  name: '@the-dev-tools/spec-lib/core',
});

export const $decorators = {
  DevTools: {
    copyParent,
    normalKey,
    parent,
  },
  'DevTools.Private': {
    copyKey,
    copyParentKey,
    omitKey,
  },
};

export class EmitterOptions extends Schema.Class<EmitterOptions>('EmitterOptions')({
  bufTypeScriptPath: Schema.String,
  dataClientPath: Schema.String,
  goPackage: pipe(Schema.String, Schema.optionalWith({ as: 'Option' })),
  rootNamespace: pipe(Schema.String, Schema.optionalWith({ default: () => 'API' })),
  version: pipe(Schema.Positive, Schema.int(), Schema.optionalWith({ default: () => 1 })),
}) {}

const { makeStateMap, makeStateSet } = makeStateFactory((_) => $lib.createStateSymbol(_));

export const normalKeys = makeStateSet<ModelProperty>('normalKeys');
export const parents = makeStateMap<Model, Model>('parents');

export const getModelKey = (program: Program, model: Model) =>
  pipe(
    $(program).model.getProperties(model),
    Array.fromIterable,
    Array.findFirst(([_key, value]) => isKey(program, value)),
  );

function parent({ program }: DecoratorContext, target: Model, parent: Model) {
  parents(program).set(target, parent);
}

function copyParent({ program }: DecoratorContext, target: Model, base: Model) {
  const parent = parents(program).get(base);
  if (parent) parents(program).set(target, parent);
}

function copyKey({ program }: DecoratorContext, target: Model, source: Model) {
  Option.gen(function* () {
    const [key, value] = yield* getModelKey(program, source);
    target.properties.set(key, value);
  });
}

function copyParentKey({ program }: DecoratorContext, target: Model, base: Model) {
  Option.gen(function* () {
    const parent = yield* Option.fromNullable(parents(program).get(base));
    const [key, value] = yield* getModelKey(program, parent);
    target.properties.set(key, value);
    normalKeys(program).add(value);
  });
}

function omitKey({ program }: DecoratorContext, target: Model) {
  Option.gen(function* () {
    const [key] = yield* getModelKey(program, target);
    target.properties.delete(key);
  });
}

function normalKey({ program }: DecoratorContext, target: ModelProperty) {
  normalKeys(program).add(target);
}
