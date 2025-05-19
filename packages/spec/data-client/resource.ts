import { DescMessage, DescMethodUnary, MessageInitShape, MessageShape } from '@bufbuild/protobuf';
import { Transport } from '@connectrpc/connect';
import { Endpoint, EntityMap, schema, Schema } from '@data-client/endpoint';
import { Equivalence, Record } from 'effect';

import { createMethodKey, createMethodKeyRecord, fetchMethod } from './utils';

interface ListProps<I extends DescMessage, O extends DescMessage, S extends Schema> {
  inputPrimaryKeys: (keyof MessageShape<I>)[];
  itemSchema: S;
  method: DescMethodUnary<I, O>;
  name: string;
}

export const list = <I extends DescMessage, O extends DescMessage, S extends Schema>({
  inputPrimaryKeys,
  itemSchema,
  method,
  name,
}: ListProps<I, O, S>) => {
  const fetchFunction = (transport: Transport, input: MessageInitShape<I>) => fetchMethod(transport, method, input);

  const key = (...[transport, input]: Parameters<typeof fetchFunction>) =>
    name + ':' + createMethodKey(transport, method, input);

  const argsKey = (...[transport, input]: Parameters<typeof fetchFunction>) =>
    createMethodKeyRecord(transport, method, input, inputPrimaryKeys);

  const items = new schema.Collection([itemSchema], { argsKey });

  return new Endpoint(fetchFunction, { key, name, schema: { items } });
};

interface GetProps<I extends DescMessage, O extends DescMessage, S extends Schema> {
  method: DescMethodUnary<I, O>;
  name: string;
  schema: S;
}

export const get = <I extends DescMessage, O extends DescMessage, S extends Schema>({
  method,
  name,
  schema,
}: GetProps<I, O, S>) => {
  const fetchFunction = (transport: Transport, input: MessageInitShape<I>) => fetchMethod(transport, method, input);

  const key = (...[transport, input]: Parameters<typeof fetchFunction>) =>
    name + ':' + createMethodKey(transport, method, input);

  return new Endpoint(fetchFunction, { key, name, schema });
};

interface CreateProps<I extends DescMessage, O extends DescMessage, S extends Schema> {
  listInputPrimaryKeys: (keyof MessageShape<I>)[];
  listItemSchema: S;
  method: DescMethodUnary<I, O>;
  name: string;
}

export const create = <I extends DescMessage, O extends DescMessage, S extends Schema>({
  listInputPrimaryKeys,
  listItemSchema,
  method,
  name,
}: CreateProps<I, O, S>) => {
  const fetchFunction = async (transport: Transport, input: MessageInitShape<I>) => {
    const output = await fetchMethod(transport, method, input);
    return { ...input, ...output };
  };

  const createCollectionFilter =
    (...[transport, input]: Parameters<typeof fetchFunction>) =>
    (collectionKey: Record<string, string>) => {
      const argsKey = createMethodKeyRecord(transport, method, input, listInputPrimaryKeys);
      const compare = Record.getEquivalence(Equivalence.string);
      return compare(argsKey, collectionKey);
    };

  const list = new schema.Collection([listItemSchema], { createCollectionFilter });

  return new Endpoint(fetchFunction, { name, schema: list.push, sideEffect: true });
};

interface UpdateProps<I extends DescMessage, O extends DescMessage, S extends Schema> {
  method: DescMethodUnary<I, O>;
  name: string;
  schema: S;
}

export const update = <I extends DescMessage, O extends DescMessage, S extends Schema>({
  method,
  name,
  schema,
}: UpdateProps<I, O, S>) => {
  const fetchFunction = async (transport: Transport, input: MessageInitShape<I>) => {
    await fetchMethod(transport, method, input);
    return input;
  };

  return new Endpoint(fetchFunction, { name, schema, sideEffect: true });
};

interface DeleteProps<I extends DescMessage, O extends DescMessage, S extends Schema> {
  method: DescMethodUnary<I, O>;
  name: string;
  schema: S;
}

export const delete$ = <
  I extends DescMessage,
  O extends DescMessage,
  S extends EntityMap[string] & { process: unknown },
>({
  method,
  name,
  schema: entitySchema,
}: DeleteProps<I, O, S>) => {
  const fetchFunction = async (transport: Transport, input: MessageInitShape<I>) => {
    await fetchMethod(transport, method, input);
    return input;
  };

  const invalidate = new schema.Invalidate(entitySchema);

  return new Endpoint(fetchFunction, { name, schema: invalidate, sideEffect: true });
};
