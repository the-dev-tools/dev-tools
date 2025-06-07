import { DescMethodUnary } from '@bufbuild/protobuf';
import { Endpoint, Schema } from '@data-client/endpoint';

import { SourceKind } from '../dist/buf/typescript/delta/v1/delta_pb';
import { UpdateProps } from './resource';
import { EndpointProps, makeEndpointFn, makeKey } from './utils';

export const deltaUpdate = <M extends DescMethodUnary, S extends Schema>({
  method,
  name,
  schema,
}: UpdateProps<M, S>) => {
  const endpointFn = async (props: EndpointProps<M, { source?: SourceKind }>) => {
    await makeEndpointFn(method)(props);
    const { source } = props.input;
    return { ...props.input, source: source === SourceKind.ORIGIN ? SourceKind.MIXED : source };
  };

  return new Endpoint(endpointFn, {
    key: makeKey(method, name),
    name,
    schema,
    sideEffect: true,
  });
};

export const deltaReset = <M extends DescMethodUnary, S extends Schema>({
  method,
  name,
  schema,
}: UpdateProps<M, S>) => {
  const endpointFn = async (props: EndpointProps<M>) => {
    await makeEndpointFn(method)(props);
    return { ...props.input, source: SourceKind.ORIGIN };
  };

  return new Endpoint(endpointFn, {
    key: makeKey(method, name),
    name,
    schema,
    sideEffect: true,
  });
};
