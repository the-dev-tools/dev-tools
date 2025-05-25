import { EndpointInstance } from '@data-client/endpoint';
import { Controller, Queryable, SchemaArgs, useController, useLoading } from '@data-client/react';

export const useMutate = <E extends EndpointInstance>(endpoint: E) => {
  const controller = useController();
  return useLoading((...data: Parameters<E['fetch']>) => controller.fetch(endpoint, ...data), [controller, endpoint]);
};

// TODO: fix types upstream
export const setQueryChild = <S extends Queryable>(
  controller: Controller,
  schema: S,
  childKey: keyof S,
  ...rest: readonly [...SchemaArgs<S>, object]
) => controller.set(schema[childKey] as S, ...rest);
