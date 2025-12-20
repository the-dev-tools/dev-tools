import { create } from '@bufbuild/protobuf';
import { eq, useLiveQuery } from '@tanstack/react-db';
import * as XF from '@xyflow/react';
import { Ulid } from 'id128';
import { use } from 'react';
import { Tooltip, TooltipTrigger } from 'react-aria-components';
import { FiExternalLink, FiX } from 'react-icons/fi';
import { NodeHttpSchema } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { NodeExecutionCollectionSchema, NodeHttpCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { HttpCollectionSchema, HttpDeltaCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { ButtonAsLink } from '@the-dev-tools/ui/button';
import { SendRequestIcon } from '@the-dev-tools/ui/icons';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/api';
import { HttpRequest, HttpResponse, HttpUrl } from '~/features/http';
import { ReferenceContext } from '~/reference';
import { httpDeltaRouteApi, httpRouteApi, workspaceRouteApi } from '~/routes';
import { useDeltaState } from '~/utils/delta';
import { pick } from '~/utils/tanstack-db';
import { FlowContext } from '../context';
import { Handle } from '../handle';
import {
  NodeBodyNew,
  NodeExecutionOutputProps,
  NodeExecutionPanel,
  NodeName,
  NodePanelProps,
  NodeStateIndicator,
  NodeTitle,
} from '../node';

export const HttpNode = ({ id, selected }: XF.NodeProps) => {
  const nodeId = Ulid.fromCanonical(id).bytes;

  return (
    <div className={tw`pointer-events-none flex flex-col items-center`}>
      <div className={tw`pointer-events-auto relative`}>
        <NodeBodyNew className={tw`text-violet-600`} icon={<SendRequestIcon />} nodeId={nodeId} selected={selected} />

        <Handle nodeId={nodeId} position={XF.Position.Left} type='target' />
        <Handle nodeId={nodeId} position={XF.Position.Right} type='source' />
      </div>

      <NodeTitle className={tw`mt-1`}>HTTP request</NodeTitle>
      <NodeName nodeId={nodeId} />
      <NodeStateIndicator nodeId={nodeId} />
    </div>
  );
};

export const HttpPanel = ({ nodeId }: NodePanelProps) => {
  const { isReadOnly = false } = use(FlowContext);

  const { workspaceId } = workspaceRouteApi.useLoaderData();
  const { workspaceIdCan } = workspaceRouteApi.useParams();

  const nodeHttpCollection = useApiCollection(NodeHttpCollectionSchema);

  const { deltaHttpId, httpId } =
    useLiveQuery(
      (_) =>
        _.from({ item: nodeHttpCollection })
          .where((_) => eq(_.item.nodeId, nodeId))
          .select((_) => pick(_.item, 'httpId', 'deltaHttpId'))
          .findOne(),
      [nodeHttpCollection, nodeId],
    ).data ?? create(NodeHttpSchema);

  const deltaOptions = {
    deltaId: deltaHttpId,
    deltaSchema: HttpDeltaCollectionSchema,
    isDelta: deltaHttpId !== undefined,
    originId: httpId,
    originSchema: HttpCollectionSchema,
  };

  const [name] = useDeltaState({ ...deltaOptions, valueKey: 'name' });

  return (
    <>
      <div className={tw`sticky top-0 z-10 flex items-center border-b border-slate-200 bg-white px-5 py-2`}>
        <div className='min-w-0'>
          <div className={tw`truncate text-sm leading-5 font-medium text-slate-800`}>{name}</div>
        </div>

        <div className={tw`flex-1`} />

        <ButtonAsLink
          className={tw`shrink-0 px-2`}
          variant='ghost'
          {...(deltaHttpId
            ? {
                params: {
                  deltaHttpIdCan: Ulid.construct(deltaHttpId).toCanonical(),
                  httpIdCan: Ulid.construct(httpId).toCanonical(),
                  workspaceIdCan,
                },
                to: httpDeltaRouteApi.id,
              }
            : {
                params: {
                  httpIdCan: Ulid.construct(httpId).toCanonical(),
                  workspaceIdCan,
                },
                to: httpRouteApi.id,
              })}
        >
          <FiExternalLink className={tw`size-4 text-slate-500`} />
          Open API
        </ButtonAsLink>

        <div className={tw`mr-3 ml-2 h-5 w-px shrink-0 bg-slate-300`} />

        <TooltipTrigger delay={750}>
          <ButtonAsLink className={tw`p-1`} search={(_) => ({ ..._, node: undefined })} to='.' variant='ghost'>
            <FiX className={tw`size-5 text-slate-500`} />
          </ButtonAsLink>
          <Tooltip className={tw`rounded-md bg-slate-800 px-2 py-1 text-xs text-white`}>Close</Tooltip>
        </TooltipTrigger>
      </div>

      <div className={tw`m-5 mb-4`}>
        <HttpUrl deltaHttpId={deltaHttpId} httpId={httpId} isReadOnly={isReadOnly} />
      </div>

      <div className={tw`mx-5 overflow-auto rounded-lg border border-slate-200`}>
        <div
          className={tw`
            border-b border-slate-200 bg-slate-50 px-3 py-2 text-md leading-5 font-medium tracking-tight text-slate-800
          `}
        >
          Request
        </div>

        <ReferenceContext value={{ flowNodeId: nodeId, httpId, workspaceId, ...(deltaHttpId && { deltaHttpId }) }}>
          <HttpRequest className={tw`p-5 pt-3`} deltaHttpId={deltaHttpId} httpId={httpId} isReadOnly={isReadOnly} />
        </ReferenceContext>
      </div>

      <NodeExecutionPanel nodeId={nodeId} Output={Output} />
    </>
  );
};

const Output = ({ nodeExecutionId }: NodeExecutionOutputProps) => {
  const collection = useApiCollection(NodeExecutionCollectionSchema);

  const { httpResponseId } =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where((_) => eq(_.item.nodeExecutionId, nodeExecutionId))
          .select((_) => pick(_.item, 'httpResponseId'))
          .findOne(),
      [collection, nodeExecutionId],
    ).data ?? {};

  if (!httpResponseId) return null;

  return <HttpResponse httpResponseId={httpResponseId} />;
};
