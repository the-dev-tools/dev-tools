import { DescMethodUnary } from '@bufbuild/protobuf';
import { Endpoint, Queryable } from '@data-client/endpoint';
import { SourceKind } from '../dist/buf/typescript/delta/v1/delta_pb';
import { UpdateProps } from './resource';
import { EndpointProps, EntitySchema, makeEndpointFn, makeKey } from './utils';

export const deltaUpdate = <M extends DescMethodUnary, S extends EntitySchema>({
  method,
  name,
  schema,
}: UpdateProps<M, S>) => {
  const endpointFn = async (props: EndpointProps<M, { source?: SourceKind }>) => {
    await makeEndpointFn(method)(props);
    const { source } = props.input;

    const snapshot = props.controller().snapshot(props.controller().getState());
    const old = snapshot.get(schema as Queryable, props.input) ?? {};

    return { ...old, ...props.input, source: source === SourceKind.ORIGIN ? SourceKind.MIXED : source };
  };

  return new Endpoint(endpointFn, {
    key: makeKey(method, name),
    name,
    schema,
    sideEffect: true,
  });
};

export const deltaReset = <M extends DescMethodUnary, S extends EntitySchema>({
  method,
  name,
  schema,
}: UpdateProps<M, S>) => {
  const endpointFn = async (props: EndpointProps<M>) => {
    await makeEndpointFn(method)(props);

    const snapshot = props.controller().snapshot(props.controller().getState());
    const old = snapshot.get(schema as Queryable, props.input) ?? {};

    return { ...old, ...props.input, source: SourceKind.ORIGIN };
  };

  return new Endpoint(endpointFn, {
    key: makeKey(method, name),
    name,
    schema,
    sideEffect: true,
  });
};
