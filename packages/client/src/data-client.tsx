import { EndpointInstance } from '@data-client/endpoint';
import { useController, useLoading } from '@data-client/react';

export const useMutate = <E extends EndpointInstance>(endpoint: E) => {
  const controller = useController();
  return useLoading((...data: Parameters<E['fetch']>) => controller.fetch(endpoint, ...data), [controller, endpoint]);
};
