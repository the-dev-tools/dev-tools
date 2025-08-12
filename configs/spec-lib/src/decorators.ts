import {
  DecoratorContext,
  EnumMember,
  Interface,
  Model,
  Operation,
  Scalar,
  StringLiteral,
  Type,
} from '@typespec/compiler';
import { $ } from '@typespec/compiler/typekit';
import { Array, Option, pipe, Record } from 'effect';
import { externals, instances, maps, streams, templateInstances, templateNames, templates } from './state.js';

const instanceOf = ({ program }: DecoratorContext, instance: Model, templateMaybe?: Model) =>
  Option.gen(function* () {
    const template = yield* pipe(
      Option.fromNullable(templateMaybe),
      Option.orElse(() => Option.fromNullable(instance.sourceModel)),
    );

    const templateDecorator = yield* Array.findFirst(
      template.decorators,
      (_) => _.decorator === $decorators.Lib.templateOf,
    );

    const base = yield* pipe(
      Array.head(templateDecorator.args),
      Option.map((_) => _.value),
      Option.filter((_) => $(program).model.is(_)),
    );

    let instanceMap = templateInstances(program).get(base);
    instanceMap ??= new Map();

    instances(program).add(instance);
    templateInstances(program).set(base, instanceMap);
    instanceMap.set(template, instance);
  });

const templateOf = ({ program }: DecoratorContext, template: Interface | Model | Operation, base: Model) => {
  const name = base.name + template.name;

  if (template.kind === 'Model') {
    const isInstance = pipe(
      template.decorators,
      Array.findFirst((_) => _.decorator === instanceOf),
      Option.isSome,
    );

    if (isInstance) return;

    let instanceMap = templateInstances(program).get(base);
    instanceMap ??= new Map();

    let instance = instanceMap.get(template);
    instance ??= $(program).model.create({
      name,
      properties: pipe($(program).model.getProperties(template), Record.fromEntries),
    });

    templateInstances(program).set(base, instanceMap);
    instanceMap.set(template, instance);
  } else {
    let names = templateNames(program).get(base);
    names ??= new Map();

    templateNames(program).set(base, names);
    names.set(template, name);
  }

  templates(program).add(template);
};

const stream = ({ program }: DecoratorContext, target: Operation, mode: EnumMember) =>
  streams(program).set(target, mode.name as never);

const external = ({ program }: DecoratorContext, target: Model, path: StringLiteral, name: StringLiteral) =>
  externals(program).set(target, [path.value, name.value]);

const _map = ({ program }: DecoratorContext, target: Scalar, key: Type, value: Type) =>
  maps(program).set(target, [key, value]);

export const $decorators = {
  Lib: {
    instanceOf,
    templateOf,
  },
  TypeSpec: {
    stream,
  },
  'TypeSpec.Private': {
    external,
    map: _map,
  },
};
