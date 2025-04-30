import { create, DescMessage, DescService, MessageShape } from '@bufbuild/protobuf';
import { createConnectTransport } from '@connectrpc/connect-web';
import { EndpointInstance } from '@data-client/endpoint';
import { useController, useLoading } from '@data-client/react';

import { registry } from '~api/meta';

// TODO: would be better to get transport from context
export const transport = createConnectTransport({
  baseUrl: 'http://localhost:8080',
  jsonOptions: { registry },
  useHttpGet: true,
});

export const toClass = <Desc extends DescMessage>(schema: Desc) =>
  function () {
    return create(schema);
  } as unknown as new () => MessageShape<Desc>;

export const useMutate = <E extends EndpointInstance>(endpoint: E) => {
  const controller = useController();
  return useLoading((...data: Parameters<E['fetch']>) => controller.fetch(endpoint, ...data), [controller, endpoint]);
};

export const methodName = <Service extends DescService>(service: Service, method: keyof Service['method']) => {
  const { name, parent } = service.method[method as keyof typeof service.method]!;
  return `${parent.typeName}/${name}`;
};
