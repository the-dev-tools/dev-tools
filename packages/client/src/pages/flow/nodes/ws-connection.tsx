import { create } from '@bufbuild/protobuf';
import { eq, useLiveQuery } from '@tanstack/react-db';
import { useRouter } from '@tanstack/react-router';
import * as XF from '@xyflow/react';
import { Ulid } from 'id128';
import { use } from 'react';
import { FiExternalLink, FiWifi } from 'react-icons/fi';
import { HandleKind, NodeWsConnectionSchema } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { NodeWsConnectionCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { WebSocketCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/web_socket';
import { ButtonAsLink } from '@the-dev-tools/ui/button';
import { FieldLabel } from '@the-dev-tools/ui/field';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { ReferenceContext, ReferenceField } from '~/features/expression';
import { WebSocketHeaderTable } from '~/pages/websocket/@x/flow';
import { useApiCollection } from '~/shared/api';
import { eqStruct, pick } from '~/shared/lib';
import { routes } from '~/shared/routes';
import { FlowContext } from '../context';
import { Handle } from '../handle';
import { NodeSettingsBody, NodeSettingsProps, SimpleNode } from '../node';

const defaultNodeWsConnection = create(NodeWsConnectionSchema);

export const WsConnectionNode = ({ id, selected }: XF.NodeProps) => {
  const nodeId = Ulid.fromCanonical(id).bytes;

  const nodeWsCollection = useApiCollection(NodeWsConnectionCollectionSchema);
  const wsCollection = useApiCollection(WebSocketCollectionSchema);

  const { websocketId } =
    useLiveQuery(
      (_) =>
        _.from({ item: nodeWsCollection })
          .where((_) => eq(_.item.nodeId, nodeId))
          .select((_) => pick(_.item, 'websocketId'))
          .findOne(),
      [nodeWsCollection, nodeId],
    ).data ?? defaultNodeWsConnection;

  const { url } =
    useLiveQuery(
      (_) =>
        _.from({ item: wsCollection })
          .where((_) => (websocketId ? eqStruct({ websocketId })(_) : eq(true, false)))
          .select((_) => pick(_.item, 'url'))
          .findOne(),
      [wsCollection, websocketId],
    ).data ?? {};

  return (
    <SimpleNode
      className={tw`w-48 text-indigo-500`}
      handles={
        <>
          <Handle nodeId={nodeId} position={XF.Position.Left} type='target' />
          <Handle label='Next' nodeId={nodeId} position={XF.Position.Right} type='source' />
          <Handle kind={HandleKind.WS_MESSAGE} nodeId={nodeId} position={XF.Position.Bottom} type='source' />
        </>
      }
      icon={<FiWifi />}
      nodeId={nodeId}
      selected={selected}
      title='WS Connection'
    >
      <div className={tw`min-w-0 flex-1`}>
        <span className={tw`rounded bg-indigo-100 px-1 text-[10px] font-semibold text-indigo-600`}>WS</span>
        <div className={tw`truncate text-xs tracking-tight text-on-neutral-low`}>{url ?? 'No URL'}</div>
      </div>
    </SimpleNode>
  );
};

export const WsConnectionSettings = ({ nodeId }: NodeSettingsProps) => {
  const router = useRouter();

  const { isReadOnly = false } = use(FlowContext);

  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();
  const { workspaceIdCan } = routes.dashboard.workspace.route.useParams();

  const nodeWsCollection = useApiCollection(NodeWsConnectionCollectionSchema);
  const wsCollection = useApiCollection(WebSocketCollectionSchema);

  const { websocketId } =
    useLiveQuery(
      (_) =>
        _.from({ item: nodeWsCollection })
          .where((_) => eq(_.item.nodeId, nodeId))
          .select((_) => pick(_.item, 'websocketId'))
          .findOne(),
      [nodeWsCollection, nodeId],
    ).data ?? defaultNodeWsConnection;

  const { url } =
    useLiveQuery(
      (_) =>
        _.from({ item: wsCollection })
          .where((_) => (websocketId ? eqStruct({ websocketId })(_) : eq(true, false)))
          .select((_) => pick(_.item, 'url'))
          .findOne(),
      [wsCollection, websocketId],
    ).data ?? {};

  return (
    <NodeSettingsBody
      nodeId={nodeId}
      settingsHeader={
        websocketId && (
          <ButtonAsLink
            className={tw`-my-4 shrink-0 px-2`}
            params={{
              websocketIdCan: Ulid.construct(websocketId).toCanonical(),
              workspaceIdCan,
            }}
            to={router.routesById[routes.dashboard.workspace.websocket.route.id].fullPath}
            variant='ghost'
          >
            <FiExternalLink className={tw`size-4 text-on-neutral-low`} />
            Open WebSocket
          </ButtonAsLink>
        )
      }
      title='WebSocket Connection'
    >
      <ReferenceContext value={{ flowNodeId: nodeId, websocketId, workspaceId }}>
        <FieldLabel>URL</FieldLabel>
        <ReferenceField
          kind='StringExpression'
          onChange={(_) => websocketId && wsCollection.utils.update({ url: _, websocketId })}
          readOnly={isReadOnly || !websocketId}
          value={url ?? ''}
        />

        {websocketId && (
          <>
            <FieldLabel className={tw`mt-4`}>Headers</FieldLabel>
            <WebSocketHeaderTable websocketId={websocketId} />
          </>
        )}
      </ReferenceContext>
    </NodeSettingsBody>
  );
};
