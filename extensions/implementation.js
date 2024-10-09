import { listOperationsIn } from '@typespec/compiler';
import { setExtension } from '@typespec/openapi';

/**
 * @param {import('@typespec/compiler').DecoratorContext} context
 * @param {import('@typespec/compiler').Type} target
 */
export function $operationGroup(context, target, value) {
  const operations = listOperationsIn(target, { recursive: true });
  operations.forEach((operation) => {
    setExtension(context.program, operation, 'x-ogen-operation-group', target.namespace.name);
  });
}
