import { eq, useLiveQuery } from '@tanstack/react-db';
import { Panel, Group as PanelGroup, useDefaultLayout } from 'react-resizable-panels';
import { GraphQLResponseCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/graph_q_l';
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { ReferenceContext } from '~/features/expression';
import { useApiCollection } from '~/shared/api';
import { pick } from '~/shared/lib';
import { routes } from '~/shared/routes';
import { GraphQLRequestPanel } from './request/panel';
import { GraphQLTopBar } from './request/top-bar';
import { GraphQLResponsePanel } from './response';

export const GraphQLPage = () => {
  const { graphqlId } = routes.dashboard.workspace.graphql.route.useRouteContext();
  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();

  const responseCollection = useApiCollection(GraphQLResponseCollectionSchema);

  const { graphqlResponseId } =
    useLiveQuery(
      (_) =>
        _.from({ item: responseCollection })
          .where((_) => eq(_.item.graphqlId, graphqlId))
          .select((_) => pick(_.item, 'graphqlResponseId'))
          .orderBy((_) => _.item.graphqlResponseId, 'desc')
          .limit(1)
          .findOne(),
      [responseCollection, graphqlId],
    ).data ?? {};

  const endpointLayout = useDefaultLayout({ id: 'graphql-endpoint' });

  return (
    <PanelGroup {...endpointLayout} orientation='vertical'>
      <Panel className='flex h-full flex-col' id='request'>
        <ReferenceContext value={{ workspaceId }}>
          <GraphQLTopBar graphqlId={graphqlId} />
          <GraphQLRequestPanel graphqlId={graphqlId} />
        </ReferenceContext>
      </Panel>

      {graphqlResponseId && (
        <>
          <PanelResizeHandle direction='vertical' />

          <Panel defaultSize='40%' id='response'>
            <GraphQLResponsePanel graphqlResponseId={graphqlResponseId} />
          </Panel>
        </>
      )}
    </PanelGroup>
  );
};
