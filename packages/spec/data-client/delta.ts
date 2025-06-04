import { DescMessage, MessageInitShape } from '@bufbuild/protobuf';
import { Transport } from '@connectrpc/connect';
import { Endpoint, Schema } from '@data-client/endpoint';

import { SourceKind } from '../dist/buf/typescript/delta/v1/delta_pb';
import { UpdateProps } from './resource';
import { createMethodKey, fetchMethod } from './utils';

export const deltaUpdate = <I extends DescMessage, O extends DescMessage, S extends Schema>({
  method,
  name,
  schema,
}: UpdateProps<I, O, S>) => {
  const fetchFunction = async (transport: Transport, input: MessageInitShape<I> & { source?: SourceKind }) => {
    await fetchMethod(transport, method, input);
    return { ...input, source: input.source === SourceKind.ORIGIN ? SourceKind.MIXED : input.source };
  };

  const key = (...[transport, input]: Parameters<typeof fetchFunction>) =>
    [name, createMethodKey(transport, method, input)].join(' ');

  return new Endpoint(fetchFunction, { key, name, schema, sideEffect: true });
};

export const deltaReset = <I extends DescMessage, O extends DescMessage, S extends Schema>({
  method,
  name,
  schema,
}: UpdateProps<I, O, S>) => {
  const fetchFunction = async (transport: Transport, input: MessageInitShape<I>) => {
    await fetchMethod(transport, method, input);
    return { ...input, source: SourceKind.ORIGIN };
  };

  const key = (...[transport, input]: Parameters<typeof fetchFunction>) =>
    [name, createMethodKey(transport, method, input)].join(' ');

  return new Endpoint(fetchFunction, { key, name, schema, sideEffect: true });
};
