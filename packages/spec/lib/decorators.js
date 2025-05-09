import { getFriendlyName, getKeyName } from '@typespec/compiler';
import { $field } from '@typespec/protobuf';
import { getParentResource, getResourceTypeKey } from '@typespec/rest';
import { Array, Hash, Number, Option, pipe, Record } from 'effect';

import { $lib } from './lib.js';

/** @import { DecoratorApplication, DecoratorContext, Model, ModelProperty, Operation, Type, Namespace } from '@typespec/compiler' */

/**
 * @param {DecoratorContext} context
 * @param {Model} target
 */
export function $copyKey(context, target) {
  const { program } = context;

  const resourceType = target.templateMapper?.args?.[0];
  if (resourceType?.kind !== 'Model') return;

  const resourceKey = getResourceTypeKey(program, resourceType);
  if (!resourceKey) return;

  const { keyProperty } = resourceKey;
  const keyName = getKeyName(program, keyProperty);
  if (!keyName) return;

  target.properties.set(keyName, keyProperty);
}

/**
 * @param {DecoratorContext} context
 * @param {Model} target
 */
export function $copyParentKey(context, target) {
  const { program } = context;

  const resourceType = target.templateMapper?.args?.[0];
  if (resourceType?.kind !== 'Model') return;

  const parentType = getParentResource(program, resourceType);
  if (!parentType) return;

  const resourceKey = getResourceTypeKey(program, parentType);
  if (!resourceKey) return;

  let { keyProperty } = resourceKey;
  const keyName = getKeyName(program, keyProperty);
  if (!keyName) return;

  const decorators = pipe(
    keyProperty.decorators,
    // Remove key decorator
    Array.filter((_) => !(_.definition?.namespace.name === 'TypeSpec' && _.definition?.name === '@key')),
    // Add normal key decorator
    Array.append(/** @type {DecoratorApplication} */ ({ decorator: $normalKey, args: [] })),
  );

  target.properties.set(keyName, { ...keyProperty, decorators });
}

/**
 * @param {DecoratorContext} context
 * @param {Model} target
 */
export function $omitKey(context, target) {
  const { program } = context;

  const resourceKey = getResourceTypeKey(program, target);
  if (!resourceKey) return;

  const keyName = getKeyName(program, resourceKey.keyProperty);
  if (!keyName) return;

  target.properties.delete(keyName);
}

/**
 * https://protobuf.dev/programming-guides/proto3/#assigning
 * @param {string} value
 */
const fieldNumberFromName = (value) => {
  const fieldNumber = Math.abs(Hash.string(value) % 536_870_911);
  if (Number.between({ minimum: 19_000, maximum: 19_999 })) return Math.trunc(fieldNumber / 10);
  return fieldNumber;
};

/**
 * @param {DecoratorContext} context
 * @param {ModelProperty} target
 */
export function $autoField(context, target) {
  const fieldNumber = fieldNumberFromName(target.name);

  context.call($field, target, fieldNumber);

  target.decorators.push({
    decorator: $field,
    args: [
      {
        value: context.program.checker.createLiteralType(fieldNumber),
        jsValue: fieldNumber,
        node: target,
      },
    ],
  });
}

/**
 * @param {DecoratorContext} context
 * @param {Model} target
 */
export function $autoFields(context, target) {
  target.properties.forEach((property) => {
    const fieldNumber = fieldNumberFromName(target.name + property.name);

    context.call($field, property, fieldNumber);

    property.decorators.push({
      decorator: $field,
      args: [
        {
          value: context.program.checker.createLiteralType(fieldNumber),
          jsValue: fieldNumber,
          node: property,
        },
      ],
    });
  });
}

/**
 * @param {DecoratorContext} context
 * @param {ModelProperty} target
 */
export function $normalKey(context, target) {
  if (!target.model) return;

  const normalKeyMap = context.program.stateMap($lib.stateKeys.normalKeys);

  if (!normalKeyMap.has(target.model)) normalKeyMap.set(target.model, []);

  /** @type {string[]} */
  const normalKeys = normalKeyMap.get(target.model);

  normalKeys.push(target.name);
}

/**
 * @param {DecoratorContext} context
 * @param {Model} target
 * @param {Model | undefined} base
 */
export function $normalize(context, target, base) {
  context.program.stateMap($lib.stateKeys.base).set(target, base ?? target);
}

/**
 * @param {DecoratorContext} context
 * @param {Model} target
 * @param {Type} value
 */
export function $autoChange(context, target, value) {
  const packageStateMap = context.program.stateMap(Symbol.for('@typespec/protobuf.package'));

  /**
   * @param {Type} type
   * @returns {unknown}
   */
  function typeToJson(type) {
    if (type.kind === 'Model') {
      return pipe(
        type.properties.entries(),
        Record.fromEntries,
        Record.filterMap((property, key) => {
          if (key !== '$type') return pipe(property.type, typeToJson, Option.some);
          if (property.type.kind !== 'Model') return Option.none();

          return pipe(
            Option.fromNullable(property.type.namespace),
            Option.flatMapNullable((_) => packageStateMap.get(_)),
            Option.flatMapNullable((/** @type {Model} */ _) => _.properties.get('name')?.type.value),
            Option.map((packageName) => `${packageName}.${property.type.name}`),
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

  /** @type {Map<Type, unknown[]>} */
  const autoChangesMap = context.program.stateMap($lib.stateKeys.autoChanges);
  pipe(autoChangesMap.get(target) ?? [], Array.append(change), (_) => autoChangesMap.set(target, _));
}

/**
 * @param {DecoratorContext} context
 * @param {Model} target
 * @param {Namespace | Model} from
 * @param {Namespace | Model} to
 */
export function $move(context, target, from, to) {
  const fromNamespace = from.kind === 'Namespace' ? from : from.namespace;
  const toNamespace = to.kind === 'Namespace' ? to : to.namespace;

  if (!toNamespace || target.namespace !== fromNamespace) return;

  context.program.stateMap($lib.stateKeys.move).set(target, toNamespace);
}

/**
 * @param {DecoratorContext} context
 * @param {Operation} target
 */
export function $useFriendlyName(context, target) {
  target.name = getFriendlyName(context.program, target) ?? target.name;
}
