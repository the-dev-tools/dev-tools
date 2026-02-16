import { create } from '@bufbuild/protobuf';
import { eq, useLiveQuery } from '@tanstack/react-db';
import { useRouter } from '@tanstack/react-router';
import * as XF from '@xyflow/react';
import { Ulid } from 'id128';
import { use } from 'react';
import { FiExternalLink } from 'react-icons/fi';
import { NodeGraphQLSchema } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import {
  NodeExecutionCollectionSchema,
  NodeGraphQLCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { GraphQLCollectionSchema, GraphQLDeltaCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/graph_q_l';
import { ButtonAsLink } from '@the-dev-tools/ui/button';
import { SendRequestIcon } from '@the-dev-tools/ui/icons';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useDeltaState } from '~/features/delta';
import { ReferenceContext } from '~/features/expression';
import { GraphQLRequestPanel, GraphQLResponseInfo, GraphQLResponsePanel } from '~/pages/graphql/@x/flow';
import { useApiCollection } from '~/shared/api';
import { pick } from '~/shared/lib';
import { routes } from '~/shared/routes';
import { FlowContext } from '../context';
import { Handle } from '../handle';
import { NodeSettingsBody, NodeSettingsOutputProps, NodeSettingsProps, SimpleNode } from '../node';

export const GraphQLNode = ({ id, selected }: XF.NodeProps) => {
  const nodeId = Ulid.fromCanonical(id).bytes;

  const nodeGraphQLCollection = useApiCollection(NodeGraphQLCollectionSchema);

  const { deltaGraphqlId, graphqlId } =
    useLiveQuery(
      (_) =>
        _.from({ item: nodeGraphQLCollection })
          .where((_) => eq(_.item.nodeId, nodeId))
          .select((_) => pick(_.item, 'graphqlId', 'deltaGraphqlId'))
          .findOne(),
      [nodeGraphQLCollection, nodeId],
    ).data ?? create(NodeGraphQLSchema);

  const deltaOptions = {
    deltaId: deltaGraphqlId,
    deltaSchema: GraphQLDeltaCollectionSchema,
    isDelta: deltaGraphqlId !== undefined,
    originId: graphqlId,
    originSchema: GraphQLCollectionSchema,
  };

  const [name] = useDeltaState({ ...deltaOptions, valueKey: 'name' });

  return (
    <SimpleNode
      className={tw`w-48 text-teal-600`}
      handles={
        <>
          <Handle nodeId={nodeId} position={XF.Position.Left} type='target' />
          <Handle nodeId={nodeId} position={XF.Position.Right} type='source' />
        </>
      }
      icon={<SendRequestIcon />}
      nodeId={nodeId}
      selected={selected}
      title='GraphQL'
    >
      <div className={tw`min-w-0 flex-1`}>
        <div className={tw`truncate text-xs font-medium tracking-tight text-teal-600`}>GQL</div>
        <div className={tw`truncate text-xs tracking-tight text-on-neutral-low`}>{name}</div>
      </div>
    </SimpleNode>
  );
};

export const GraphQLSettings = ({ nodeId }: NodeSettingsProps) => {
  const router = useRouter();

  const { isReadOnly = false } = use(FlowContext);

  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();
  const { workspaceIdCan } = routes.dashboard.workspace.route.useParams();

  const nodeGraphQLCollection = useApiCollection(NodeGraphQLCollectionSchema);

  const { deltaGraphqlId, graphqlId } =
    useLiveQuery(
      (_) =>
        _.from({ item: nodeGraphQLCollection })
          .where((_) => eq(_.item.nodeId, nodeId))
          .select((_) => pick(_.item, 'graphqlId', 'deltaGraphqlId'))
          .findOne(),
      [nodeGraphQLCollection, nodeId],
    ).data ?? create(NodeGraphQLSchema);

  return (
    <NodeSettingsBody
      nodeId={nodeId}
      output={(_) => <Output nodeExecutionId={_} />}
      settingsHeader={
        <ButtonAsLink
          className={tw`-my-4 shrink-0 px-2`}
          variant='ghost'
          {...(deltaGraphqlId
            ? {
                params: {
                  deltaGraphqlIdCan: Ulid.construct(deltaGraphqlId).toCanonical(),
                  graphqlIdCan: Ulid.construct(graphqlId).toCanonical(),
                  workspaceIdCan,
                },
                to: router.routesById[routes.dashboard.workspace.graphql.delta.id].fullPath,
              }
            : {
                params: {
                  graphqlIdCan: Ulid.construct(graphqlId).toCanonical(),
                  workspaceIdCan,
                },
                to: router.routesById[routes.dashboard.workspace.graphql.route.id].fullPath,
              })}
        >
          <FiExternalLink className={tw`size-4 text-on-neutral-low`} />
          Open GraphQL
        </ButtonAsLink>
      }
      title='GraphQL request'
    >
      <ReferenceContext
        value={{ flowNodeId: nodeId, graphqlId, workspaceId, ...(deltaGraphqlId && { deltaGraphqlId }) }}
      >
        <GraphQLRequestPanel
          deltaGraphqlId={deltaGraphqlId}
          graphqlId={graphqlId}
          isReadOnly={isReadOnly}
        />
      </ReferenceContext>
    </NodeSettingsBody>
  );
};

const Output = ({ nodeExecutionId }: NodeSettingsOutputProps) => {
  const collection = useApiCollection(NodeExecutionCollectionSchema);

  const { graphqlResponseId } =
    useLiveQuery(
      (_) =>
        _.from({ item: collection })
          .where((_) => eq(_.item.nodeExecutionId, nodeExecutionId))
          .select((_) => pick(_.item, 'graphqlResponseId'))
          .findOne(),
      [collection, nodeExecutionId],
    ).data ?? {};

  if (!graphqlResponseId) return null;

  return (
    <div className={tw`flex h-full flex-col`}>
      <GraphQLResponseInfo className={tw`-m-2`} graphqlResponseId={graphqlResponseId} />
      <GraphQLResponsePanel className={tw`flex-1`} graphqlResponseId={graphqlResponseId} />
    </div>
  );
};
