import {
  createTypeSpecLibrary,
  DecoratorContext,
  Interface,
  Model,
  Operation,
  Program,
  Type,
} from '@typespec/compiler';
import { $ } from '@typespec/compiler/typekit';
import { Array, Option, pipe, Record } from 'effect';

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

const stateKeys = ['instances', 'instancesByModel', 'instancesByTemplate', 'templates'] as const;
const state: Record<(typeof stateKeys)[number], symbol> = Record.fromIterableWith(stateKeys, (_) => [
  _,
  $lib.createStateSymbol(_),
]);

export const instances = (program: Program) => program.stateSet(state.instances);

export const instancesByModel = (program: Program) =>
  program.stateMap(state.instancesByModel) as Map<Model, Set<Model>>;

export const instancesByTemplate = (program: Program) =>
  program.stateMap(state.instancesByTemplate) as Map<Type, Model>;

export const templates = (program: Program) => program.stateSet(state.templates);

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
