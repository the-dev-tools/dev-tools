import { useLiveQuery } from '@tanstack/react-db';
import { useEffect } from 'react';
import {
  GraphQLCollectionSchema,
  GraphQLDeltaCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/graph_q_l';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useDeltaState } from '~/features/delta';
import { useApiCollection } from '~/shared/api';
import { eqStruct } from '~/shared/lib';
import { routes } from '~/shared/routes';
import { useCloseTab } from '~/widgets/tabs';

export interface GraphQLTabProps {
  deltaGraphqlId?: Uint8Array;
  graphqlId: Uint8Array;
}

export const graphqlTabId = ({ deltaGraphqlId, graphqlId }: GraphQLTabProps) =>
  JSON.stringify({ deltaGraphqlId, graphqlId, route: routes.dashboard.workspace.graphql.route.id });

export const GraphQLTab = ({ deltaGraphqlId, graphqlId }: GraphQLTabProps) => {
  const closeTab = useCloseTab();

  const graphqlCollection = useApiCollection(GraphQLCollectionSchema);

  const graphqlExists =
    useLiveQuery(
      (_) => _.from({ item: graphqlCollection }).where(eqStruct({ graphqlId })).findOne(),
      [graphqlCollection, graphqlId],
    ).data !== undefined;

  useEffect(() => {
    if (!graphqlExists) void closeTab(graphqlTabId({ graphqlId }));
  }, [graphqlExists, graphqlId, closeTab]);

  const deltaCollection = useApiCollection(GraphQLDeltaCollectionSchema);

  const deltaExists =
    useLiveQuery(
      (_) => _.from({ item: deltaCollection }).where(eqStruct({ deltaGraphqlId })).findOne(),
      [deltaCollection, deltaGraphqlId],
    ).data !== undefined;

  useEffect(() => {
    if (deltaGraphqlId && !deltaExists) void closeTab(graphqlTabId({ deltaGraphqlId, graphqlId }));
  }, [deltaExists, deltaGraphqlId, graphqlId, closeTab]);

  const deltaOptions = {
    deltaId: deltaGraphqlId,
    deltaSchema: GraphQLDeltaCollectionSchema,
    isDelta: deltaGraphqlId !== undefined,
    originId: graphqlId,
    originSchema: GraphQLCollectionSchema,
  };

  const [name] = useDeltaState({ ...deltaOptions, valueKey: 'name' });

  return (
    <>
      <span className={tw`rounded bg-pink-100 px-1.5 py-0.5 text-[10px] font-semibold text-pink-700`}>GQL</span>
      <span className={tw`min-w-0 flex-1 truncate`}>{name}</span>
    </>
  );
};
