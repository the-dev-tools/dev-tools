import { Endpoint } from '@data-client/endpoint';
import { ImportService } from '../dist/buf/typescript/import/v1/import_pb';
import { MakeEndpointProps } from './resource';
import { makeEndpointFn, makeKey } from './utils';

export const import$ = ({ method, name }: MakeEndpointProps<typeof ImportService.method.import>) =>
  new Endpoint(makeEndpointFn(method), {
    key: makeKey(method, name),
    name,
    sideEffect: true,
  });
