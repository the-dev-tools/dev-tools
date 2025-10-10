import { Endpoint, makeEndpointFn, makeKey } from '@the-dev-tools/spec-lib/data-client/utils.ts';
import { ImportService } from '../dist/buf/typescript/import/v1/import_pb';
import { MakeEndpointProps } from './resource';

export const import$ = ({ method, name }: MakeEndpointProps<typeof ImportService.method.import>) =>
  new Endpoint(makeEndpointFn(method), {
    key: makeKey(method, name),
    name,
    sideEffect: true,
  });
