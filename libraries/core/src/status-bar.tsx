import { createClient } from '@connectrpc/connect';
import { createConnectQueryKey } from '@connectrpc/connect-query';
import { useQuery } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import { Array } from 'effect';
import { Ulid } from 'id128';
import { useMemo } from 'react';
import { FiTerminal, FiTrash2, FiX } from 'react-icons/fi';
import { Panel } from 'react-resizable-panels';
import { twMerge } from 'tailwind-merge';

import { LogService, LogStreamResponse } from '@the-dev-tools/spec/log/v1/log_pb';
import { Button, ButtonAsLink } from '@the-dev-tools/ui/button';
import { ArrowToLeftIcon } from '@the-dev-tools/ui/icons';
import { PanelResizeHandle, panelResizeHandleStyles } from '@the-dev-tools/ui/resizable-panel';
import { tw } from '@the-dev-tools/ui/tailwind-literal';

import type { WorkspaceRouteSearch } from './workspace-layout';

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceIdCan');

export const useLogsQuery = () => {
  const { transport, queryClient } = workspaceRoute.useRouteContext();

  const { logStream } = useMemo(() => createClient(LogService, transport), [transport]);

  const queryKey = useMemo(
    () =>
      createConnectQueryKey({
        schema: { ...LogService.method.logStream, methodKind: 'unary' },
        cardinality: 'infinite',
        transport,
      }),
    [transport],
  );

  const query = useQuery({
    queryKey,
    initialData: [],
    meta: { normalize: false },
    queryFn: async ({ queryKey, signal }) => {
      for await (const log of logStream({})) {
        queryClient.setQueryData(queryKey, Array.append(log));
        if (signal.aborted) break;
      }
      return queryClient.getQueryData<LogStreamResponse[]>(queryKey)!;
    },
  });

  return { ...query, queryKey };
};

export const StatusBar = () => {
  const { showLogs } = workspaceRoute.useSearch();
  const { queryClient } = workspaceRoute.useRouteContext();

  const { queryKey, data: logs } = useLogsQuery();

  const separator = <div className={tw`h-3.5 w-px bg-slate-200`} />;

  const bar = (
    <div className={twMerge(tw`flex items-center gap-2 bg-slate-50 px-2 py-1`, showLogs && tw`bg-white`)}>
      {/* TODO: implement sidebar collapse */}
      <Button variant='ghost' className={tw`p-0.5`}>
        <ArrowToLeftIcon className={tw`size-4 text-slate-500`} />
      </Button>

      {separator}

      <ButtonAsLink
        variant='ghost'
        className={tw`px-2 py-1 text-xs leading-4 tracking-tight text-slate-800`}
        href={{
          to: '.',
          search: (_: Partial<WorkspaceRouteSearch>) =>
            ({ ..._, showLogs: true }) satisfies Partial<WorkspaceRouteSearch>,
        }}
      >
        <FiTerminal className={tw`size-3`} />
        <span>Logs</span>
      </ButtonAsLink>

      <div className={tw`flex-1`} />

      {showLogs && (
        <>
          <Button
            variant='ghost'
            className={tw`px-2 py-1 text-xs leading-4 tracking-tight text-slate-800`}
            onPress={() => void queryClient.setQueryData(queryKey, [])}
          >
            <FiTrash2 className={tw`size-3 text-slate-500`} />
            <span>Clear Logs</span>
          </Button>

          {separator}

          <ButtonAsLink
            variant='ghost'
            className={tw`p-0.5`}
            href={{
              to: '.',
              search: (_: Partial<WorkspaceRouteSearch>) =>
                ({ ..._, showLogs: undefined }) satisfies Partial<WorkspaceRouteSearch>,
            }}
          >
            <FiX className={tw`size-4 text-slate-500`} />
          </ButtonAsLink>
        </>
      )}
    </div>
  );

  return (
    <>
      {showLogs ? (
        <PanelResizeHandle direction='vertical' />
      ) : (
        <div className={panelResizeHandleStyles({ direction: 'vertical' })} />
      )}

      {bar}

      {showLogs && (
        <Panel id='status' order={100} className={tw`p-4 pt-0`}>
          <div
            className={tw`flex size-full flex-col-reverse overflow-auto rounded-md border border-slate-200 bg-slate-800 p-3 font-mono text-sm leading-5 text-slate-200 shadow-sm`}
          >
            <div>
              {logs.map((_) => {
                const ulid = Ulid.construct(_.logId);
                return (
                  <div key={ulid.toCanonical()}>
                    {ulid.time.toLocaleTimeString()}: {_.value}
                  </div>
                );
              })}
            </div>
          </div>
        </Panel>
      )}
    </>
  );
};
