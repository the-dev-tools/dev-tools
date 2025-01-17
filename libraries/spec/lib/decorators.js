import { getKeyName } from '@typespec/compiler';
import { $field } from '@typespec/protobuf';
import { getParentResource, getResourceTypeKey } from '@typespec/rest';
import { Array, Hash, Number, pipe } from 'effect';

import { $lib } from './lib.js';

/** @import { DecoratorApplication, DecoratorContext, Model, ModelProperty } from '@typespec/compiler' */

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

  const normalKeyMap = context.program.stateMap($lib.stateKeys.normalKey);

  if (!normalKeyMap.has(target.model)) normalKeyMap.set(target.model, []);

  /** @type {string[]} */
  const normalKeys = normalKeyMap.get(target.model);

  normalKeys.push(target.name);
}
