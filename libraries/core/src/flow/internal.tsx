import { getRouteApi } from '@tanstack/react-router';
import { Handle as HandleCore, HandleProps } from '@xyflow/react';

import {
  Handle as HandleKind,
  HandleJson as HandleKindJson,
  HandleSchema as HandleKindSchema,
} from '@the-dev-tools/spec/flow/edge/v1/edge_pb';
import { tw } from '@the-dev-tools/ui/tailwind-literal';

export { HandleKind, HandleKindSchema, type HandleKindJson };

export const Handle = (props: HandleProps) => (
  <HandleCore
    className={tw`-z-10 flex size-5 items-center justify-center rounded-full border border-slate-300 bg-slate-200 shadow-sm`}
    {...props}
  >
    <div className={tw`pointer-events-none size-2 rounded-full bg-slate-800`} />
  </HandleCore>
);

export const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceIdCan');
export const flowRoute = getRouteApi('/_authorized/workspace/$workspaceIdCan/flow/$flowIdCan');
