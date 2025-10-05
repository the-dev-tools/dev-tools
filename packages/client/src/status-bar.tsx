import { createClient } from '@connectrpc/connect';
import { experimental_streamedQuery as streamedQuery, useQuery } from '@tanstack/react-query';
import { Array, pipe } from 'effect';
import { Ulid } from 'id128';
import { Tree as AriaTree } from 'react-aria-components';
import { FiTerminal, FiTrash2, FiX } from 'react-icons/fi';
import { Panel } from 'react-resizable-panels';
import { twMerge } from 'tailwind-merge';
import { tv } from 'tailwind-variants';
import { LogLevel, LogService, LogStreamResponse, LogStreamResponseSchema } from '@the-dev-tools/spec/log/v1/log_pb';
import { Button, ButtonAsLink } from '@the-dev-tools/ui/button';
import { JsonTreeItem, jsonTreeItemProps } from '@the-dev-tools/ui/json-tree';
import { PanelResizeHandle, panelResizeHandleStyles } from '@the-dev-tools/ui/resizable-panel';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TreeItem } from '@the-dev-tools/ui/tree';
import { ConnectStreamingQueryKey, createConnectStreamingQueryKey } from '~api/connect-query';
import { workspaceRouteApi } from '~routes';

export const useLogsQuery = () => {
  const { transport } = workspaceRouteApi.useRouteContext();

  const { logStream } = createClient(LogService, transport);

  const queryKey: ConnectStreamingQueryKey<typeof LogStreamResponseSchema> = createConnectStreamingQueryKey({
    schema: LogService.method.logStream,
    transport,
  });

  const query = useQuery({
    queryFn: streamedQuery({
      initialValue: Array.empty<LogStreamResponse>(),
      reducer: (acc, value) => pipe(Array.append(acc, value), Array.takeRight(100)),
      refetchMode: 'append',
      streamFn: () => logStream({}),
    }),
    queryKey,
  });

  return { ...query, queryKey };
};

const logTextStyles = tv({
  base: tw`font-mono text-sm`,
  variants: {
    level: {
      [LogLevel.ERROR]: tw`text-red-600`,
      [LogLevel.UNSPECIFIED]: tw`text-slate-800`,
      [LogLevel.WARNING]: tw`text-yellow-600`,
    } satisfies Record<LogLevel, string>,
  },
});

export const StatusBar = () => {
  const { showLogs } = workspaceRouteApi.useSearch();
  const { queryClient } = workspaceRouteApi.useRouteContext();

  const { data: logs, queryKey } = useLogsQuery();

  const separator = <div className={tw`h-3.5 w-px bg-slate-200`} />;

  const bar = (
    <div className={twMerge(tw`flex items-center gap-2 bg-slate-50 px-2 py-1`, showLogs && tw`bg-white`)}>
      <ButtonAsLink
        className={tw`px-2 py-1 text-xs leading-4 tracking-tight text-slate-800`}
        search={(_) => ({ ..._, showLogs: showLogs ? undefined : true })}
        to='.'
        variant='ghost'
      >
        <FiTerminal className={tw`size-3`} />
        <span>Logs</span>
      </ButtonAsLink>

      <div className={tw`flex-1`} />

      <div id='statusBarEndSlot' />

      {showLogs && (
        <>
          <Button
            className={tw`px-2 py-1 text-xs leading-4 tracking-tight text-slate-800`}
            onPress={() => void queryClient.setQueryData(queryKey, [])}
            variant='ghost'
          >
            <FiTrash2 className={tw`size-3 text-slate-500`} />
            <span>Clear Logs</span>
          </Button>

          {separator}

          <ButtonAsLink className={tw`p-0.5`} search={(_) => ({ ..._, showLogs: undefined })} to='.' variant='ghost'>
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
        <Panel>
          <div className={tw`flex size-full flex-col-reverse overflow-auto`}>
            <AriaTree aria-label='Logs' items={logs ?? []}>
              {(_) => {
                const ulid = Ulid.construct(_.logId);
                const id = ulid.toCanonical();
                return (
                  <TreeItem
                    id={id}
                    item={(_) => <JsonTreeItem {..._} id={`${id}.root`} />}
                    items={jsonTreeItemProps(_.value)!}
                    textValue={_.name}
                  >
                    <div className={logTextStyles({ level: _.level })}>
                      {ulid.time.toLocaleTimeString()}: {_.name}
                    </div>
                  </TreeItem>
                );
              }}
            </AriaTree>
          </div>
        </Panel>
      )}
    </>
  );
};
