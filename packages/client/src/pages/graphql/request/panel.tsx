import { count, eq, useLiveQuery } from '@tanstack/react-db';
import { Suspense } from 'react';
import { Tab, TabList, TabPanel, Tabs } from 'react-aria-components';
import { twMerge } from 'tailwind-merge';
import { GraphQLHeaderCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/graph_q_l';
import { Spinner } from '@the-dev-tools/ui/spinner';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/shared/api';
import { GraphQLHeaderTable } from './header';
import { GraphQLQueryEditor } from './query-editor';
import { GraphQLVariablesEditor } from './variables-editor';

export interface GraphQLRequestPanelProps {
  graphqlId: Uint8Array;
}

export const GraphQLRequestPanel = ({ graphqlId }: GraphQLRequestPanelProps) => {
  const headerCollection = useApiCollection(GraphQLHeaderCollectionSchema);

  const { headerCount = 0 } =
    useLiveQuery(
      (_) =>
        _.from({ item: headerCollection })
          .where((_) => eq(_.item.graphqlId, graphqlId))
          .select((_) => ({ headerCount: count(_.item.graphqlId) }))
          .findOne(),
      [headerCollection, graphqlId],
    ).data ?? {};

  const tabClass = ({ isSelected }: { isSelected: boolean }) =>
    twMerge(
      tw`
        -mb-px cursor-pointer border-b-2 border-transparent py-1.5 text-md leading-5 font-medium tracking-tight
        text-on-neutral-low transition-colors
      `,
      isSelected && tw`border-b-accent text-on-neutral`,
    );

  return (
    <Tabs className={tw`flex flex-1 flex-col gap-6 overflow-auto p-6 pt-4`} defaultSelectedKey='query'>
      <TabList className={tw`flex gap-3 border-b border-neutral`}>
        <Tab className={tabClass} id='query'>
          Query
        </Tab>

        <Tab className={tabClass} id='variables'>
          Variables
        </Tab>

        <Tab className={tabClass} id='headers'>
          Headers
          {headerCount > 0 && <span className={tw`text-xs text-success`}> ({headerCount})</span>}
        </Tab>
      </TabList>

      <Suspense
        fallback={
          <div className={tw`flex h-full items-center justify-center`}>
            <Spinner size='lg' />
          </div>
        }
      >
        <TabPanel className={tw`h-full`} id='query'>
          <GraphQLQueryEditor graphqlId={graphqlId} />
        </TabPanel>

        <TabPanel className={tw`h-full`} id='variables'>
          <GraphQLVariablesEditor graphqlId={graphqlId} />
        </TabPanel>

        <TabPanel id='headers'>
          <GraphQLHeaderTable graphqlId={graphqlId} />
        </TabPanel>
      </Suspense>
    </Tabs>
  );
};
