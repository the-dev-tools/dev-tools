import { DescMethodUnary } from '@bufbuild/protobuf';
import { createContextValues, Transport } from '@connectrpc/connect';
import {
  Controller,
  Denormalize,
  DenormalizeNullable,
  EndpointInterface,
  Queryable,
  ResolveType,
  Schema,
  SchemaArgs,
  useDLE as useBaseDLE,
  useLoading,
  useSuspense,
} from '@data-client/react';
import { useRouteContext } from '@tanstack/react-router';
import { Option, pipe } from 'effect';
import { EndpointProps } from '@the-dev-tools/spec/data-client/utils';
import { enableErrorInterceptorKey } from '~api/transport';

export const useMutate = <E extends EndpointInterface<(props: EndpointProps<DescMethodUnary>) => Promise<unknown>>>(
  endpoint: E,
) => {
  const { dataClient } = useRouteContext({ from: '__root__' });

  return useLoading(
    (input: Parameters<E>[0]['input'], params?: Partial<Omit<Parameters<E>[0], 'input'>>) =>
      dataClient.fetch(endpoint, input, params),
    [dataClient, endpoint],
  );
};

export const useDLE = <
  E extends EndpointInterface<(props: EndpointProps<DescMethodUnary>) => Promise<unknown>, Schema, false>,
>(
  endpoint: E,
  input: null | Parameters<E>[0]['input'],
  params?: Partial<Omit<Parameters<E>[0], 'input'>>,
) => {
  const { transport } = useRouteContext({ from: '__root__' });

  const args = pipe(
    Option.fromNullable(input),
    Option.map((input) => [{ input, transport, ...params }] as Parameters<E>),
    Option.getOrElse(() => [null] as const),
  );

  return useBaseDLE(endpoint, ...args);
};

// TODO: fix types upstream
export const setQueryChild = <S extends Queryable>(
  controller: Controller,
  schema: S,
  childKey: keyof S,
  ...rest: readonly [...SchemaArgs<S>, object]
) => controller.set(schema[childKey] as S, ...rest);

export const useQuery = <
  E extends EndpointInterface<(props: EndpointProps<DescMethodUnary>) => Promise<unknown>, Schema, false>,
  I extends null | Parameters<E>[0]['input'],
>(
  endpoint: E,
  input: I,
  params?: Partial<Omit<Parameters<E>[0], 'input'>>,
) => {
  const { transport } = useRouteContext({ from: '__root__' });

  const args = pipe(
    Option.fromNullable(input),
    Option.map((input) => [{ input, transport, ...params }] as Parameters<E>),
    Option.getOrElse(() => [null] as const),
  );

  type ResultSure = E['schema'] extends null ? ResolveType<E> : Denormalize<E['schema']>;
  type ResultMaybe = E['schema'] extends null ? ResolveType<E> | undefined : DenormalizeNullable<E['schema']>;
  type Result = I extends null ? ResultMaybe : ResultSure;

  // eslint-disable-next-line @typescript-eslint/no-unsafe-return
  return useSuspense(endpoint, ...args) as Result;
};

interface MakeDataClientProps {
  controller: Controller;
  transport: Transport;
}

export const makeDataClient = ({ controller, transport }: MakeDataClientProps) => {
  const fetch = <E extends EndpointInterface<(props: EndpointProps<DescMethodUnary>) => Promise<unknown>>>(
    endpoint: E,
    input: Parameters<E>[0]['input'],
    params?: Partial<Omit<Parameters<E>[0], 'input'>>,
  ) => {
    const contextValues = params?.contextValues ?? createContextValues();
    contextValues.set(enableErrorInterceptorKey, true);
    return controller.fetch(endpoint, ...([{ input, transport, ...params, contextValues }] as Parameters<E>));
  };

  return { controller, fetch };
};

export interface DataClient extends ReturnType<typeof makeDataClient> {}
