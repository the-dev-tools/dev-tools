import { DescMessage, MessageInitShape } from '@bufbuild/protobuf';
import { Transport } from '@connectrpc/connect';
import { Endpoint } from '@data-client/endpoint';

import { EndpointProps } from './resource';
import { createMethodKey, fetchMethod } from './utils';

const placeholder = <I extends DescMessage, O extends DescMessage>({ method, name }: EndpointProps<I, O>) => {
  const fetchFunction = (transport: Transport, input: MessageInitShape<I>) => fetchMethod(transport, method, input);

  const key = (...[transport, input]: Parameters<typeof fetchFunction>) =>
    [name, createMethodKey(transport, method, input)].join(' ');

  return new Endpoint(fetchFunction, { key, name });
};

// TODO: implement delta endpoints
export const deltaList = placeholder;
export const deltaCreate = placeholder;
export const deltaUpdate = placeholder;
export const deltaReset = placeholder;
