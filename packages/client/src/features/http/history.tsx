import { eq, useLiveQuery } from '@tanstack/react-db';
import { Ulid } from 'id128';
import { Suspense } from 'react';
import { Collection, Dialog, Tab, TabList, TabPanel, Tabs } from 'react-aria-components';
import { Panel, PanelGroup } from 'react-resizable-panels';
import { twJoin } from 'tailwind-merge';
import { HttpResponseCollectionSchema, HttpVersionCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { Modal } from '@the-dev-tools/ui/modal';
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { Spinner } from '@the-dev-tools/ui/spinner';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/api';
import { pick } from '~/utils/tanstack-db';
import { HttpRequestPanel } from './request/panel';
import { HttpUrl } from './request/url';
import { ResponsePanel } from './response';

export interface HistoryModalProps {
  deltaHttpId: Uint8Array | undefined;
  httpId: Uint8Array;
}

export const HistoryModal = ({ deltaHttpId, httpId }: HistoryModalProps) => {
  'use no memo';

  const collection = useApiCollection(HttpVersionCollectionSchema);

  const { data: versions } = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where((_) => eq(_.item.httpId, deltaHttpId ?? httpId))
        .orderBy((_) => _.item.httpVersionId, 'desc'),
    [collection, deltaHttpId, httpId],
  );

  return (
    <Modal isDismissable size='lg'>
      <Dialog className={tw`size-full outline-hidden`}>
        <Tabs className={tw`flex h-full`} orientation='vertical'>
          <div className={tw`flex w-64 flex-col border-r border-slate-200 bg-slate-50 p-4 tracking-tight`}>
            <div className={tw`mb-4`}>
              <div className={tw`mb-0.5 text-sm leading-5 font-semibold text-slate-800`}>Response History</div>
              <div className={tw`text-xs leading-4 text-slate-500`}>History of your API response</div>
            </div>
            <div className={tw`grid min-h-0 grid-cols-[auto_1fr] gap-x-0.5`}>
              <div className={tw`flex flex-col items-center gap-0.5`}>
                <div className={tw`flex-1`} />
                <div className={tw`size-2 rounded-full border border-violet-700 p-px`}>
                  <div className={tw`size-full rounded-full border border-inherit`} />
                </div>
                <div className={tw`w-px flex-1 bg-slate-200`} />
              </div>

              <div className={tw`p-2 text-md leading-5 font-semibold tracking-tight text-violet-700`}>
                Current Version
              </div>

              <div className={tw`flex flex-col items-center gap-0.5`}>
                <div className={tw`w-px flex-1 bg-slate-200`} />
                <div className={tw`size-2 rounded-full bg-slate-300`} />
                <div className={tw`w-px flex-1 bg-slate-200`} />
              </div>

              <div className={tw`p-2 text-md leading-5 font-semibold tracking-tight text-slate-800`}>
                {versions.length} previous responses
              </div>

              <div className={tw`mb-2 w-px flex-1 justify-self-center bg-slate-200`} />

              <TabList className={tw`overflow-auto`} items={versions}>
                {(_) => (
                  <Tab
                    className={({ isSelected }) =>
                      twJoin(
                        tw`
                          flex cursor-pointer items-center gap-1.5 rounded-md px-3 py-1.5 text-md leading-5
                          font-semibold text-slate-800
                        `,
                        isSelected && tw`bg-slate-200`,
                      )
                    }
                    id={collection.utils.getKey(_)}
                  >
                    {Ulid.construct(_.httpVersionId).time.toLocaleString()}
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
                    <Version deltaHttpId={deltaHttpId} httpId={_.httpVersionId} />
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
  deltaHttpId: Uint8Array | undefined;
  httpId: Uint8Array;
}

const Version = ({ deltaHttpId, httpId }: VersionProps) => {
  const responseCollection = useApiCollection(HttpResponseCollectionSchema);

  const { data: { httpResponseId } = {} } = useLiveQuery(
    (_) =>
      _.from({ item: responseCollection })
        .where((_) => eq(_.item.httpId, httpId))
        .select((_) => pick(_.item, 'httpResponseId'))
        .orderBy((_) => _.item.httpResponseId, 'desc')
        .limit(1)
        .findOne(),
    [responseCollection, httpId],
  );

  return (
    <PanelGroup autoSaveId='endpoint-versions' direction='vertical'>
      <Panel className={tw`flex h-full flex-col`} id='request' order={1}>
        <div className={tw`p-6 pb-2`}>
          <HttpUrl deltaHttpId={deltaHttpId} httpId={httpId} isReadOnly />
        </div>

        <HttpRequestPanel deltaHttpId={deltaHttpId} httpId={httpId} isReadOnly />
      </Panel>

      {httpResponseId && (
        <>
          <PanelResizeHandle direction='vertical' />

          <Panel defaultSize={40} id='response' order={2}>
            <ResponsePanel fullWidth httpResponseId={httpResponseId} />
          </Panel>
        </>
      )}
    </PanelGroup>
  );
};
