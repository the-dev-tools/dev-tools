import { DescMethodUnary, MessageShape } from '@bufbuild/protobuf';
import { Endpoint, EntityMap, schema, Schema } from '@data-client/endpoint';
import { Equivalence, Record } from 'effect';

import { createMethodKeyRecord, EndpointProps, makeEndpointFn, makeKey } from './utils';

export interface MakeEndpointProps<M extends DescMethodUnary> {
  method: M;
  name: string;
}

interface ListProps<M extends DescMethodUnary, S extends Schema> extends MakeEndpointProps<M> {
  inputPrimaryKeys: (keyof MessageShape<M['input']>)[];
  itemSchema: S;
}

export const list = <M extends DescMethodUnary, S extends Schema>({
  inputPrimaryKeys,
  itemSchema,
  method,
  name,
}: ListProps<M, S>) => {
  const argsKey = (props: EndpointProps<M> | null) => {
    if (props === null) return {};
    const { input, transport } = props;
    return createMethodKeyRecord(transport, method, input, inputPrimaryKeys);
  };

  const createCollectionFilter =
    ({ input, transport }: EndpointProps<M>) =>
    (collectionKey: Record<string, string>) => {
      const argsKey = createMethodKeyRecord(transport, method, input, inputPrimaryKeys);
      const compare = Record.getEquivalence(Equivalence.string);
      return compare(argsKey, collectionKey);
    };

  const items = new schema.Collection([itemSchema], { argsKey, createCollectionFilter });

  return new Endpoint(makeEndpointFn(method), {
    key: makeKey(method, name),
    name,
    schema: { items },
  });
};

interface GetProps<M extends DescMethodUnary, S extends Schema> extends MakeEndpointProps<M> {
  schema: S;
}

export const get = <M extends DescMethodUnary, S extends Schema>({ method, name, schema }: GetProps<M, S>) =>
  new Endpoint(makeEndpointFn(method), {
    key: makeKey(method, name),
    name,
    schema,
  });

interface CreateProps<M extends DescMethodUnary, S extends Schema> extends MakeEndpointProps<M> {
  listInputPrimaryKeys: (keyof MessageShape<M['input']>)[];
  listItemSchema: S;
}

export const create = <M extends DescMethodUnary, S extends Schema>({
  listInputPrimaryKeys,
  listItemSchema,
  method,
  name,
}: CreateProps<M, S>) => {
  const endpointFn = async (props: EndpointProps<M>) => {
    const output = await makeEndpointFn(method)(props);
    return { ...props.input, ...output };
  };

  const createCollectionFilter =
    ({ input, transport }: EndpointProps<M>) =>
    (collectionKey: Record<string, string>) => {
      const argsKey = createMethodKeyRecord(transport, method, input, listInputPrimaryKeys);
      const compare = Record.getEquivalence(Equivalence.string);
      return compare(argsKey, collectionKey);
    };

  const list = new schema.Collection([listItemSchema], { createCollectionFilter });

  return new Endpoint(endpointFn, {
    key: makeKey(method, name),
    name,
    schema: list.push,
    sideEffect: true,
  });
};

export interface UpdateProps<M extends DescMethodUnary, S extends Schema> extends MakeEndpointProps<M> {
  schema: S;
}

export const update = <M extends DescMethodUnary, S extends Schema>({ method, name, schema }: UpdateProps<M, S>) => {
  const endpointFn = async (props: EndpointProps<M>) => {
    await makeEndpointFn(method)(props);
    return props.input;
  };

  return new Endpoint(endpointFn, {
    key: makeKey(method, name),
    name,
    schema,
    sideEffect: true,
  });
};

interface DeleteProps<M extends DescMethodUnary, S extends Schema> extends MakeEndpointProps<M> {
  schema: S;
}

export const delete$ = <M extends DescMethodUnary, S extends EntityMap[string] & { process: unknown }>({
  method,
  name,
  schema: entitySchema,
}: DeleteProps<M, S>) => {
  const endpointFn = async (props: EndpointProps<M>) => {
    await makeEndpointFn(method)(props);
    return props.input;
  };

  return new Endpoint(endpointFn, {
    key: makeKey(method, name),
    name,
    schema: new schema.Invalidate(entitySchema),
    sideEffect: true,
  });
};
