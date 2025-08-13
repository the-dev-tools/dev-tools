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
import { addInstance, externals, instancesByTemplate, maps, streams, templates } from './state.js';

// Lib

const instanceOf = ({ program }: DecoratorContext, instance: Model, templateMaybe: Model) =>
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

    addInstance({ base, instance, override: true, program, template });
  });

const templateOf = (context: DecoratorContext, template: Interface | Model | Operation, base?: Model) => {
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
    template.operations.forEach((_) => {
      _.name = base.name + _.name;
    });
  }

  templates(program).add(template);
};

// TypeSpec

const stream = ({ program }: DecoratorContext, target: Operation, mode: EnumMember) =>
  streams(program).set(target, mode.name as never);

// TypeSpec.Private

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
