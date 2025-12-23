import { create } from '@bufbuild/protobuf';
import { eq, useLiveQuery } from '@tanstack/react-db';
import * as XF from '@xyflow/react';
import { Ulid } from 'id128';
import { use } from 'react';
import { FiExternalLink } from 'react-icons/fi';
import { NodeHttpSchema } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { NodeExecutionCollectionSchema, NodeHttpCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { ButtonAsLink } from '@the-dev-tools/ui/button';
import { SendRequestIcon } from '@the-dev-tools/ui/icons';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/api';
import { HttpRequest, HttpResponse, HttpUrl } from '~/features/http';
import { ReferenceContext } from '~/reference';
import { httpDeltaRouteApi, httpRouteApi, workspaceRouteApi } from '~/routes';
import { pick } from '~/utils/tanstack-db';
import { FlowContext } from '../context';
import { Handle } from '../handle';
import {
  NodeBodyNew,
  NodeExecutionOutputProps,
  NodeName,
  NodePanelProps,
  NodeSettings,
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

export const HttpSettings = ({ nodeId }: NodePanelProps) => {
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

  return (
    <NodeSettings
      nodeId={nodeId}
      output={(_) => <Output nodeExecutionId={_} />}
      settingsHeader={
        <ButtonAsLink
          className={tw`-my-4 shrink-0 px-2`}
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
      }
      title='HTTP request'
    >
      <HttpUrl deltaHttpId={deltaHttpId} httpId={httpId} isReadOnly={isReadOnly} />

      <ReferenceContext value={{ flowNodeId: nodeId, httpId, workspaceId, ...(deltaHttpId && { deltaHttpId }) }}>
        <HttpRequest className={tw`px-0`} deltaHttpId={deltaHttpId} httpId={httpId} isReadOnly={isReadOnly} />
      </ReferenceContext>
    </NodeSettings>
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
