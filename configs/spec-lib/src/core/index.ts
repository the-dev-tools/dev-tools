import { refkey, Refkey } from '@alloy-js/core';
import {
  createTypeSpecLibrary,
  DecoratorContext,
  Interface,
  isKey,
  isTemplateDeclaration,
  Model,
  ModelProperty,
  Namespace,
  Program,
} from '@typespec/compiler';
import { $ } from '@typespec/compiler/typekit';
import { Array, HashMap, Option, pipe, Schema } from 'effect';
import { makeStateFactory } from '../utils.js';

export const $lib = createTypeSpecLibrary({
  diagnostics: {},
  name: '@the-dev-tools/spec-lib/core',
});

export const $decorators = {
  DevTools: {
    copyParent,
    instanceOf,
    normalKey,
    parent,
    templateOf,
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

const modelDerivations = makeStateMap<Model, Set<Model>>('modelDerivations');
const templateMap = {
  toBase: makeStateMap<Model, Model>('templateMap.toBase'),

  fromInstance: makeStateMap<Model, Model>('templateMap.fromInstance'),
  toInstance: makeStateMap<Model, Model>('templateMap.toInstance'),
};

export const getModelKey = (program: Program, model: Model) =>
  pipe(
    $(program).model.getProperties(model),
    Array.fromIterable,
    Array.findFirst(([_key, value]) => isKey(program, value)),
  );

export const getModelProperties = (program: Program, target: Model): HashMap.HashMap<string, ModelProperty> => {
  const model = templateMap.toInstance(program).get(target) ?? target;
  const baseProperties = $(program).model.getProperties(model, { includeExtended: true });

  return pipe(
    model.sourceModels,
    Array.filter((_) => _.model !== target && _.model !== model),
    Array.flatMap((_) => pipe(getModelProperties(program, _.model), HashMap.toEntries)),
    Array.prependAll(baseProperties),
    HashMap.fromIterable,
  );
};

export const getModelName = (program: Program, target: Model): string => {
  const instance = templateMap.toInstance(program).get(target) ?? target;

  if (templateMap.fromInstance(program).has(instance)) return instance.name;

  let name = instance.name;

  const base = templateMap.toBase(program).get(instance);
  if (base) name = getModelName(program, base) + name;

  return name;
};

export const getModelNamespace = (program: Program, target: Model): Namespace => {
  const instance = templateMap.toInstance(program).get(target);
  if (instance?.namespace) return instance.namespace;

  const base = templateMap.toBase(program).get(target);
  if (base) return getModelNamespace(program, base);

  if (!target.namespace) throw Error('No namespace found');
  return target.namespace;
};

export const getModelRefKey = (program: Program, target: Model): Refkey => {
  const model = templateMap.fromInstance(program).get(target) ?? target;

  if (!model.templateNode) return refkey(model);

  const base = pipe(
    templateMap.toBase(program).get(model),
    Option.fromNullable,
    Option.map((_) => getModelRefKey(program, _)),
    Option.getOrThrow,
  );

  return refkey(model.templateNode, base);
};

export const getModelDerivations = (program: Program, target: Model): Model[] => {
  if (!target.isFinished) $(program).type.finishType(target);
  if (isTemplateDeclaration(target)) return [];

  return pipe(
    modelDerivations(program).get(target)?.values().toArray() ?? [],
    Array.flatMap((_) => getModelDerivations(program, _)),
    Array.prepend(target),
    Array.filter((_) => !templateMap.toInstance(program).has(target)),
  );
};

const addDerivation = (program: Program, template: Model, base: Model) => {
  templateMap.toBase(program).set(template, base);

  const derivations = modelDerivations(program).get(base) ?? new Set();
  derivations.add(template);
  modelDerivations(program).set(base, derivations);
};

function templateOf(context: DecoratorContext, template: Interface | Model, base: Model) {
  const { program } = context;

  if (template.kind === 'Model') {
    if (template.sourceModel) instanceOf(context, template, template.sourceModel);

    addDerivation(program, template, base);
  }

  if (template.kind === 'Interface') {
    // Avoid recursively renaming extended interfaces
    const extendedOperationCount = template.sourceInterfaces.reduce(
      (count, _interface) => count + _interface.operations.size,
      0,
    );

    const getBaseName = (target: Model): string => {
      let name = target.name;
      const base = templateMap.toBase(program).get(target);
      if (base) name = getBaseName(base) + name;
      return name;
    };

    pipe(
      template.operations.values(),
      Array.fromIterable,
      Array.drop(extendedOperationCount),
      Array.forEach((_) => (_.name = getBaseName(base) + _.name)),
    );
  }
}

function instanceOf({ program }: DecoratorContext, instance: Model, template: Model) {
  templateMap.toInstance(program).set(template, instance);
  templateMap.fromInstance(program).set(instance, template);

  const base = pipe(templateMap.toBase(program).get(template), Option.fromNullable, Option.getOrThrow);
  addDerivation(program, instance, base);
}

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
