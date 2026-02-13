import { create } from '@bufbuild/protobuf';
import { eq, useLiveQuery } from '@tanstack/react-db';
import { useRouter } from '@tanstack/react-router';
import * as XF from '@xyflow/react';
import { Ulid } from 'id128';
import { use } from 'react';
import { FiExternalLink } from 'react-icons/fi';
import { NodeHttpSchema } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { HttpMethod } from '@the-dev-tools/spec/buf/api/http/v1/http_pb';
import { NodeExecutionCollectionSchema, NodeHttpCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { HttpCollectionSchema, HttpDeltaCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { ButtonAsLink } from '@the-dev-tools/ui/button';
import { SendRequestIcon } from '@the-dev-tools/ui/icons';
import { MethodBadge } from '@the-dev-tools/ui/method-badge';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useDeltaState } from '~/features/delta';
import { ReferenceContext } from '~/features/expression';
import { HttpRequest, HttpResponseInfo, HttpResponsePanel, HttpUrl } from '~/pages/http/@x/flow';
import { useApiCollection } from '~/shared/api';
import { pick } from '~/shared/lib';
import { routes } from '~/shared/routes';
import { FlowContext } from '../context';
import { Handle } from '../handle';
import { NodeSettingsBody, NodeSettingsOutputProps, NodeSettingsProps, SimpleNode } from '../node';

export const HttpNode = ({ id, selected }: XF.NodeProps) => {
  const nodeId = Ulid.fromCanonical(id).bytes;

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
  const [method] = useDeltaState({ ...deltaOptions, valueKey: 'method' });

  return (
    <SimpleNode
      className={tw`w-48 text-accent`}
      handles={
        <>
          <Handle nodeId={nodeId} position={XF.Position.Left} type='target' />
          <Handle nodeId={nodeId} position={XF.Position.Right} type='source' />
        </>
      }
      icon={<SendRequestIcon />}
      nodeId={nodeId}
      selected={selected}
      title='HTTP Request'
    >
      <div className={tw`min-w-0 flex-1`}>
        <MethodBadge className={tw`border`} method={method ?? HttpMethod.UNSPECIFIED} />

        <div className={tw`truncate text-xs tracking-tight text-on-neutral-low`}>{name}</div>
      </div>
    </SimpleNode>
  );
};

export const HttpSettings = ({ nodeId }: NodeSettingsProps) => {
  const router = useRouter();

  const { isReadOnly = false } = use(FlowContext);

  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();
  const { workspaceIdCan } = routes.dashboard.workspace.route.useParams();

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
    <NodeSettingsBody
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
                to: router.routesById[routes.dashboard.workspace.http.delta.id].fullPath,
              }
            : {
                params: {
                  httpIdCan: Ulid.construct(httpId).toCanonical(),
                  workspaceIdCan,
                },
                to: router.routesById[routes.dashboard.workspace.http.route.id].fullPath,
              })}
        >
          <FiExternalLink className={tw`size-4 text-on-neutral-low`} />
          Open API
        </ButtonAsLink>
      }
      title='HTTP request'
    >
      <HttpUrl deltaHttpId={deltaHttpId} httpId={httpId} isReadOnly={isReadOnly} />

      <ReferenceContext value={{ flowNodeId: nodeId, httpId, workspaceId, ...(deltaHttpId && { deltaHttpId }) }}>
        <HttpRequest
          className={tw`px-0`}
          deltaHttpId={deltaHttpId}
          hideDescription
          httpId={httpId}
          isReadOnly={isReadOnly}
        />
      </ReferenceContext>
    </NodeSettingsBody>
  );
};

const Output = ({ nodeExecutionId }: NodeSettingsOutputProps) => {
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

  return (
    <div className={tw`flex h-full flex-col`}>
      <HttpResponseInfo className={tw`-m-2`} httpResponseId={httpResponseId} />
      <HttpResponsePanel className={tw`flex-1`} httpResponseId={httpResponseId} />
    </div>
  );
};
