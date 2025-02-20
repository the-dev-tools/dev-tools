import { createFileRoute } from '@tanstack/react-router';
import { ReactFlowProvider } from '@xyflow/react';
import { Ulid } from 'id128';
import { Suspense, useRef } from 'react';
import { useTab, useTabList, useTabPanel } from 'react-aria';
import { Panel, PanelGroup } from 'react-resizable-panels';
import { Item, Node, TabListState, useTabListState } from 'react-stately';
import { twJoin } from 'tailwind-merge';

import { useConnectSuspenseQuery } from '@the-dev-tools/api/connect-query';
import { FlowVersionsItem } from '@the-dev-tools/spec/flow/v1/flow_pb';
import { flowVersions } from '@the-dev-tools/spec/flow/v1/flow-FlowService_connectquery';
import { Spinner } from '@the-dev-tools/ui/icons';
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { tw } from '@the-dev-tools/ui/tailwind-literal';

import { StatusBar } from '../status-bar';
import { Flow, TopBar } from './flow';

export const Route = createFileRoute('/_authorized/workspace/$workspaceIdCan/flow/$flowIdCan/history')({
  component: RouteComponent,
});

function RouteComponent() {
  const { flowIdCan } = Route.useParams();
  const flowId = Ulid.fromCanonical(flowIdCan).bytes;

  const {
    data: { items },
  } = useConnectSuspenseQuery(flowVersions, { flowId });

  const state = useTabListState({
    items,
    children: ({ flowId }) => (
      <Item key={Ulid.construct(flowId).toCanonical()}>
        <Suspense
          fallback={
            <div className={tw`flex h-full items-center justify-center`}>
              <Spinner className={tw`size-16`} />
            </div>
          }
        >
          <Flow key={Ulid.construct(flowId).toCanonical()} flowId={flowId} isReadOnly />
        </Suspense>
      </Item>
    ),
  });
  const tabListRef = useRef(null);
  const { tabListProps } = useTabList({ items, orientation: 'vertical' }, state, tabListRef);

  return (
    <>
      <Panel id='main' order={2}>
        <PanelGroup direction='vertical'>
          <ReactFlowProvider>
            <TopBar />
            <Panel id='flow' order={1} className='flex h-full flex-col'>
              <TabPanel state={state} />
            </Panel>
          </ReactFlowProvider>
          <StatusBar />
        </PanelGroup>
      </Panel>

      <PanelResizeHandle direction='horizontal' />

      <Panel
        id='history'
        order={3}
        className={tw`flex flex-col bg-slate-50 p-4 tracking-tight`}
        style={{ overflowY: 'auto' }}
        defaultSize={20}
        minSize={10}
        maxSize={40}
      >
        <div className={tw`mb-4`}>
          <div className={tw`mb-0.5 text-sm font-semibold leading-5 text-slate-800`}>Flow History</div>
          <div className={tw`text-xs leading-4 text-slate-500`}>History of your flow responses</div>
        </div>
        <div className={tw`grid grid-cols-[auto_1fr] gap-x-0.5`}>
          <div className={tw`flex flex-col items-center gap-0.5`}>
            <div className={tw`flex-1`} />
            <div className={tw`size-2 rounded-full border border-violet-700 p-px`}>
              <div className={tw`size-full rounded-full border border-inherit`} />
            </div>
            <div className={tw`w-px flex-1 bg-slate-200`} />
          </div>

          <div className={tw`p-2 text-md font-semibold leading-5 tracking-tight text-violet-700`}>Current Version</div>

          <div className={tw`flex flex-col items-center gap-0.5`}>
            <div className={tw`w-px flex-1 bg-slate-200`} />
            <div className={tw`size-2 rounded-full bg-slate-300`} />
            <div className={tw`w-px flex-1 bg-slate-200`} />
          </div>

          <div className={tw`p-2 text-md font-semibold leading-5 tracking-tight text-slate-800`}>
            {items.length} previous responses
          </div>

          <div className={tw`mb-2 w-px flex-1 justify-self-center bg-slate-200`} />

          <div ref={tabListRef} {...tabListProps}>
            {[...state.collection].map((item) => (
              <Tab key={item.key} item={item} state={state} />
            ))}
          </div>
        </div>
      </Panel>
    </>
  );
}

interface TabProps {
  item: Node<FlowVersionsItem>;
  state: TabListState<FlowVersionsItem>;
}

const Tab = ({ item, state }: TabProps) => {
  const { key, value } = item;
  const ref = useRef(null);
  const { tabProps, isSelected } = useTab({ key }, state, ref);
  if (!value) return null;
  return (
    <div
      {...tabProps}
      ref={ref}
      className={twJoin(
        tabProps.className,
        tw`flex cursor-pointer items-center gap-1.5 rounded-md px-3 py-1.5 text-md font-semibold leading-5 text-slate-800`,
        isSelected && tw`bg-slate-200`,
      )}
    >
      {Ulid.construct(value.flowId).time.toLocaleString()}
    </div>
  );
};

interface TabPanelProps {
  state: TabListState<FlowVersionsItem>;
}

const TabPanel = ({ state }: TabPanelProps) => {
  const ref = useRef(null);
  const { tabPanelProps } = useTabPanel({}, state, ref);
  return (
    <div {...tabPanelProps} ref={ref} className={tw`size-full`}>
      {state.selectedItem?.rendered}
    </div>
  );
};
