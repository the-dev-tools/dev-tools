import { useLiveQuery } from '@tanstack/react-db';
import { useEffect } from 'react';
import { GraphQLCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/graph_q_l';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/shared/api';
import { eqStruct } from '~/shared/lib';
import { routes } from '~/shared/routes';
import { useCloseTab } from '~/widgets/tabs';

export interface GraphQLTabProps {
  graphqlId: Uint8Array;
}

export const graphqlTabId = ({ graphqlId }: GraphQLTabProps) =>
  JSON.stringify({ graphqlId, route: routes.dashboard.workspace.graphql.route.id });

export const GraphQLTab = ({ graphqlId }: GraphQLTabProps) => {
  const closeTab = useCloseTab();

  const collection = useApiCollection(GraphQLCollectionSchema);

  const item = useLiveQuery(
    (_) => _.from({ item: collection }).where(eqStruct({ graphqlId })).findOne(),
    [collection, graphqlId],
  ).data;

  useEffect(() => {
    if (!item) void closeTab(graphqlTabId({ graphqlId }));
  }, [item, graphqlId, closeTab]);

  return (
    <>
      <span className={tw`rounded bg-pink-100 px-1.5 py-0.5 text-[10px] font-semibold text-pink-700`}>GQL</span>
      <span className={tw`min-w-0 flex-1 truncate`}>{item?.name}</span>
    </>
  );
};
