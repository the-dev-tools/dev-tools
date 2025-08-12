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
import { pipe, Record } from 'effect';
import { externals, maps, streams, templateInstances, templateNames, templates } from './state.js';

const templateOf = ({ program }: DecoratorContext, template: Interface | Model | Operation, base: Model) => {
  templates(program).add(template);

  const name = base.name + template.name;

  if (template.kind === 'Model') {
    let instances = templateInstances(program).get(base);
    instances ??= new Map();

    let instance = instances.get(template);
    instance ??= $(program).model.create({
      name,
      properties: pipe($(program).model.getProperties(template), Record.fromEntries),
    });

    templateInstances(program).set(base, instances);
    instances.set(template, instance);
  } else {
    let names = templateNames(program).get(base);
    names ??= new Map();

    templateNames(program).set(base, names);
    names.set(template, name);
  }
};

const stream = ({ program }: DecoratorContext, target: Operation, mode: EnumMember) =>
  streams(program).set(target, mode.name as never);

const external = ({ program }: DecoratorContext, target: Model, path: StringLiteral, name: StringLiteral) =>
  externals(program).set(target, [path.value, name.value]);

const _map = ({ program }: DecoratorContext, target: Scalar, key: Type, value: Type) =>
  maps(program).set(target, [key, value]);

export const $decorators = {
  Lib: {
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
