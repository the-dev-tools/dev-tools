import { getKeyName } from '@typespec/compiler';
import { $field } from '@typespec/protobuf';
import { getParentResource, getResourceTypeKey } from '@typespec/rest';
import { Hash, Number } from 'effect';

/**
 * @param {import('@typespec/compiler').DecoratorContext} context
 * @param {import('@typespec/compiler').Model} target
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
 * @param {import('@typespec/compiler').DecoratorContext} context
 * @param {import('@typespec/compiler').Model} target
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

  /** @type {import('@typespec/compiler').ModelProperty} */
  keyProperty = {
    ...keyProperty,
    // Remove key decorator
    decorators: keyProperty.decorators.filter(
      (_) => !(_.definition?.namespace.name === 'TypeSpec' && _.definition?.name === '@key'),
    ),
  };

  target.properties.set(keyName, keyProperty);
}

/**
 * @param {import('@typespec/compiler').DecoratorContext} context
 * @param {import('@typespec/compiler').Model} target
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
 * @param {import('@typespec/compiler').DecoratorContext} context
 * @param {import('@typespec/compiler').ModelProperty} target
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
 * @param {import('@typespec/compiler').DecoratorContext} context
 * @param {import('@typespec/compiler').Model} target
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
