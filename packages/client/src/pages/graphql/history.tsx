import { eq, useLiveQuery } from '@tanstack/react-db';
import { Ulid } from 'id128';
import { Suspense } from 'react';
import { Collection, Dialog, Tab, TabList, TabPanel, Tabs } from 'react-aria-components';
import { Panel, Group as PanelGroup, useDefaultLayout } from 'react-resizable-panels';
import { twJoin } from 'tailwind-merge';
import {
  GraphQLResponseCollectionSchema,
  GraphQLVersionCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/graph_q_l';
import { Modal } from '@the-dev-tools/ui/modal';
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { Spinner } from '@the-dev-tools/ui/spinner';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/shared/api';
import { pick } from '~/shared/lib';
import { GraphQLRequestPanel } from './request/panel';
import { GraphQLUrl } from './request/url';
import { GraphQLResponseInfo, GraphQLResponsePanel } from './response';

export interface HistoryModalProps {
  deltaGraphqlId?: Uint8Array | undefined;
  graphqlId: Uint8Array;
}

export const HistoryModal = ({ deltaGraphqlId, graphqlId }: HistoryModalProps) => {
  'use no memo';

  const collection = useApiCollection(GraphQLVersionCollectionSchema);

  const { data: versions } = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where((_) => eq(_.item.graphqlId, deltaGraphqlId ?? graphqlId))
        .orderBy((_) => _.item.graphqlVersionId, 'desc'),
    [collection, deltaGraphqlId, graphqlId],
  );

  return (
    <Modal isDismissable size='lg'>
      <Dialog className={tw`size-full outline-hidden`}>
        <Tabs className={tw`flex h-full`} orientation='vertical'>
          <div className={tw`flex w-64 flex-col border-r border-neutral bg-neutral-lower p-4 tracking-tight`}>
            <div className={tw`mb-4`}>
              <div className={tw`mb-0.5 text-sm leading-5 font-semibold text-on-neutral`}>Response History</div>
              <div className={tw`text-xs leading-4 text-on-neutral-low`}>History of your GraphQL responses</div>
            </div>
            <div className={tw`grid min-h-0 grid-cols-[auto_1fr] gap-x-0.5`}>
              <div className={tw`flex flex-col items-center gap-0.5`}>
                <div className={tw`flex-1`} />
                <div className={tw`size-2 rounded-full border border-accent p-px`}>
                  <div className={tw`size-full rounded-full border border-inherit`} />
                </div>
                <div className={tw`w-px flex-1 bg-neutral`} />
              </div>

              <div className={tw`p-2 text-md leading-5 font-semibold tracking-tight text-accent`}>Current Version</div>

              <div className={tw`flex flex-col items-center gap-0.5`}>
                <div className={tw`w-px flex-1 bg-neutral`} />
                <div className={tw`size-2 rounded-full bg-neutral-high`} />
                <div className={tw`w-px flex-1 bg-neutral`} />
              </div>

              <div className={tw`p-2 text-md leading-5 font-semibold tracking-tight text-on-neutral`}>
                {versions.length} previous responses
              </div>

              <div className={tw`mb-2 w-px flex-1 justify-self-center bg-neutral`} />

              <TabList className={tw`overflow-auto`} items={versions}>
                {(_) => (
                  <Tab
                    className={({ isSelected }) =>
                      twJoin(
                        tw`
                          flex cursor-pointer items-center gap-1.5 rounded-md px-3 py-1.5 text-md leading-5
                          font-semibold text-on-neutral
                        `,
                        isSelected && tw`bg-neutral`,
                      )
                    }
                    id={collection.utils.getKey(_)}
                  >
                    {Ulid.construct(_.graphqlVersionId).time.toLocaleString()}
                  </Tab>
                )}
              </TabList>
            </div>
          </div>

          <div className={tw`flex h-full min-w-0 flex-1 flex-col`}>
            <Collection items={versions}>
              {(_) => (
                <TabPanel className={tw`h-full`} id={collection.utils.getKey(_)}>
                  <Suspense
                    fallback={
                      <div className={tw`flex h-full items-center justify-center`}>
                        <Spinner size='lg' />
                      </div>
                    }
                  >
                    <Version graphqlId={_.graphqlVersionId} />
                  </Suspense>
                </TabPanel>
              )}
            </Collection>
          </div>
        </Tabs>
      </Dialog>
    </Modal>
  );
};

interface VersionProps {
  graphqlId: Uint8Array;
}

const Version = ({ graphqlId }: VersionProps) => {
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

  const endpointVersionsLayout = useDefaultLayout({ id: 'endpoint-versions' });

  return (
    <PanelGroup {...endpointVersionsLayout} orientation='vertical'>
      <Panel className={tw`flex h-full flex-col`} id='request'>
        <div className={tw`p-6 pb-2`}>
          <GraphQLUrl graphqlId={graphqlId} isReadOnly />
        </div>

        <GraphQLRequestPanel graphqlId={graphqlId} isReadOnly />
      </Panel>

      {graphqlResponseId && (
        <>
          <PanelResizeHandle direction='vertical' />

          <Panel defaultSize='40%' id='response'>
            <GraphQLResponsePanel fullWidth graphqlResponseId={graphqlResponseId}>
              <GraphQLResponseInfo graphqlResponseId={graphqlResponseId} />
            </GraphQLResponsePanel>
          </Panel>
        </>
      )}
    </PanelGroup>
  );
};
