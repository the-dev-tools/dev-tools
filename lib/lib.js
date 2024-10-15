import { getKeyName } from '@typespec/compiler';
import { getParentResource, getResourceTypeKey } from '@typespec/rest';

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

  const { keyProperty } = resourceKey;
  const keyName = getKeyName(program, keyProperty);
  if (!keyName) return;

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
