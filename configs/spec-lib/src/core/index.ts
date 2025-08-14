import {
  createTypeSpecLibrary,
  DecoratorContext,
  Interface,
  Model,
  ModelProperty,
  Operation,
  Program,
  Type,
} from '@typespec/compiler';
import { $ } from '@typespec/compiler/typekit';
import { Array, Option, pipe, Record } from 'effect';
import { makeStateFactory } from '../utils.js';

export const $lib = createTypeSpecLibrary({
  diagnostics: {},
  name: '@the-dev-tools/spec-lib/core',
});

export const $decorators = {
  DevTools: {
    instanceOf,
    templateOf,
  },
};

const { makeStateMap, makeStateSet } = makeStateFactory((_) => $lib.createStateSymbol(_));

export const instances = makeStateSet<Type>('instances');
export const instancesByModel = makeStateMap<Model, Set<Model>>('instancesByModel');
export const instancesByTemplate = makeStateMap<Type, Model>('instancesByTemplate');
export const normalKeys = makeStateSet<ModelProperty>('normalKeys');
export const parents = makeStateMap<Model, Model>('parents');
export const templates = makeStateSet<Type>('templates');

interface AddInstanceProps {
  base: Model;
  instance: Model;
  override?: boolean;
  program: Program;
  template: Model;
}

const addInstance = ({ base, instance, override, program, template }: AddInstanceProps) => {
  if (instances(program).has(instance) && override !== true) return;

  let baseInstances = instancesByModel(program).get(base);
  baseInstances ??= new Set();

  const oldInstance = instancesByTemplate(program).get(template);
  if (oldInstance) baseInstances.delete(oldInstance);

  baseInstances.add(instance);
  instances(program).add(instance);
  instancesByModel(program).set(base, baseInstances);
  instancesByTemplate(program).set(template, instance);
};

function instanceOf({ program }: DecoratorContext, instance: Model, template: Model) {
  Option.gen(function* () {
    const templateDecorator = yield* Array.findFirst(template.decorators, (_) => _.decorator === templateOf);

    const base = yield* pipe(
      Array.head(templateDecorator.args),
      Option.map((_) => _.value),
      Option.filter((_) => $(program).model.is(_)),
    );

    addInstance({ base, instance, override: true, program, template });
  });
}

function templateOf(context: DecoratorContext, template: Interface | Model | Operation, base?: Model) {
  const { program } = context;

  if (templates(program).has(template)) return;

  base = pipe(
    Option.fromNullable(base),
    Option.flatMapNullable((_) => instancesByTemplate(program).get(_)),
    Option.orElseSome(() => base),
    Option.getOrUndefined,
  );

  if (base && template.kind === 'Model') {
    if (template.sourceModel && instancesByTemplate(program).has(template.sourceModel))
      return void instanceOf(context, template, template.sourceModel);

    const instance = $(program).model.create({
      name: base.name + template.name,
      properties: pipe($(program).model.getProperties(template), Record.fromEntries),
    });

    addInstance({ base, instance, program, template });
  } else if (base && template.kind === 'Interface') {
    // Avoid recursively renaming extended interfaces
    const extendedOperationCount = template.sourceInterfaces.reduce(
      (count, _interface) => count + _interface.operations.size,
      0,
    );

    pipe(
      template.operations.values(),
      Array.fromIterable,
      Array.drop(extendedOperationCount),
      Array.forEach((_) => (_.name = base.name + _.name)),
    );
  }

  templates(program).add(template);
}
