import {
  DecoratorApplication,
  DecoratorContext,
  getFriendlyName,
  getKeyName,
  isType,
  Model,
  ModelProperty,
  Namespace,
  Operation,
  StringLiteral,
  Type,
} from '@typespec/compiler';
import { $field } from '@typespec/protobuf';
import { getParentResource, getResourceTypeKey } from '@typespec/rest';
import { Array, Hash, Number, Option, pipe, Record } from 'effect';

import {
  autoChangesMap,
  baseMap,
  endpointMap,
  entityMap,
  moveMap,
  normalKeySet,
  packageMap,
  parentResourceMap,
} from './state.js';

export function $copyKey({ program }: DecoratorContext, target: Model) {
  const resourceType = target.templateMapper?.args[0];
  if (!resourceType || !isType(resourceType) || resourceType.kind !== 'Model') return;

  const resourceKey = getResourceTypeKey(program, resourceType);
  if (!resourceKey) return;

  const { keyProperty } = resourceKey;
  const keyName = getKeyName(program, keyProperty);
  if (!keyName) return;

  target.properties.set(keyName, keyProperty);
}

export function $copyParentKey({ program }: DecoratorContext, target: Model) {
  const resourceType = target.templateMapper?.args[0];
  if (!resourceType || !isType(resourceType) || resourceType.kind !== 'Model') return;

  const parentType = getParentResource(program, resourceType);
  if (!parentType) return;

  const resourceKey = getResourceTypeKey(program, parentType);
  if (!resourceKey) return;

  const { keyProperty } = resourceKey;
  const keyName = getKeyName(program, keyProperty);
  if (!keyName) return;

  const decorators = pipe(
    keyProperty.decorators,
    // Remove key decorator
    Array.filter((_) => !(_.definition?.namespace.name === 'TypeSpec' && _.definition.name === '@key')),
    // Add normal key decorator
    Array.append<DecoratorApplication>({ args: [], decorator: $normalKey }),
  );

  target.properties.set(keyName, { ...keyProperty, decorators });
}

export function $parentResourceOf({ program }: DecoratorContext, target: Model, base: Model) {
  const parent = getParentResource(program, base);
  if (!parent) return;

  parentResourceMap(program).set(target, parent);
}

export function $omitKey({ program }: DecoratorContext, target: Model) {
  const resourceKey = getResourceTypeKey(program, target);
  if (!resourceKey) return;

  const keyName = getKeyName(program, resourceKey.keyProperty);
  if (!keyName) return;

  target.properties.delete(keyName);
}

// https://protobuf.dev/programming-guides/proto3/#assigning
const fieldNumberFromName = (value: string) => {
  const fieldNumber = Math.abs(Hash.string(value) % 536_870_911);
  if (Number.between(fieldNumber, { maximum: 19_999, minimum: 19_000 })) return Math.trunc(fieldNumber / 10);
  return fieldNumber;
};

export function $autoField(context: DecoratorContext, target: ModelProperty) {
  if (!target.node) return;

  const fieldNumber = fieldNumberFromName(target.name);

  context.call($field, target, fieldNumber);

  target.decorators.push({
    args: [
      {
        jsValue: fieldNumber,
        node: target.node,
        value: context.program.checker.createLiteralType(fieldNumber),
      },
    ],
    decorator: $field,
  });
}

export function $autoFields(context: DecoratorContext, target: Model) {
  target.properties.forEach((property) => {
    if (!property.node) return;

    const fieldNumber = fieldNumberFromName(target.name + property.name);

    context.call($field, property, fieldNumber);

    property.decorators.push({
      args: [
        {
          jsValue: fieldNumber,
          node: property.node,
          value: context.program.checker.createLiteralType(fieldNumber),
        },
      ],
      decorator: $field,
    });
  });
}

export function $normalKey({ program }: DecoratorContext, target: ModelProperty) {
  normalKeySet(program).add(target);
}

export function $normalize({ program }: DecoratorContext, target: Model, base?: Model) {
  baseMap(program).set(target, base ?? target);
}

export function $autoChange({ program }: DecoratorContext, target: Model, value: Type) {
  function typeToJson(type: Type): unknown {
    if (type.kind === 'Model') {
      return pipe(
        type.properties.entries(),
        Record.fromEntries,
        Record.filterMap((property, key) => {
          if (key !== '$type') return pipe(property.type, typeToJson, Option.some);
          if (property.type.kind !== 'Model') return Option.none();

          return pipe(
            Option.fromNullable(property.type.namespace),
            Option.flatMapNullable((_) => packageMap(program).get(_)?.properties.get('name')?.type),
            Option.flatMap((_) => {
              if (_.kind !== 'String') return Option.none();
              if (property.type.kind !== 'Model') return Option.none();
              return Option.some(`${_.value}.${property.type.name}`);
            }),
          );
        }),
      );
    }

    if (type.kind === 'Tuple') return Array.map(type.values, typeToJson);
    if (type.kind === 'EnumMember') return type.name;
    if ('value' in type) return type.value;

    return undefined;
  }

  const change = typeToJson(value);

  pipe(autoChangesMap(program).get(target) ?? [], Array.append(change), (_) => autoChangesMap(program).set(target, _));
}

export function $move(
  { program }: DecoratorContext,
  target: Model | Operation,
  from: Model | Namespace,
  to: Model | Namespace,
) {
  const fromNamespace = from.kind === 'Namespace' ? from : from.namespace;
  const toNamespace = to.kind === 'Namespace' ? to : to.namespace;

  if (!toNamespace || target.namespace !== fromNamespace) return;

  moveMap(program).set(target, toNamespace);
}

export function $useFriendlyName({ program }: DecoratorContext, target: Model | Operation) {
  target.name = getFriendlyName(program, target) ?? target.name;
}

export function $entity({ program }: DecoratorContext, target: Model, base?: Model) {
  entityMap(program).set(target, base ?? target);
}

export function $endpoint({ program }: DecoratorContext, target: Operation, method: StringLiteral, options?: Model) {
  endpointMap(program).set(target, { method: method.value, options });
}
