import { createClient } from '@connectrpc/connect';
import { ConnectQueryKey, createConnectQueryKey } from '@connectrpc/connect-query';
import { experimental_streamedQuery as streamedQuery, useQuery } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import { Ulid } from 'id128';
import { useMemo } from 'react';
import {
  Collection as AriaCollection,
  Tree as AriaTree,
  TreeItemContent as AriaTreeItemContent,
} from 'react-aria-components';
import { FiTerminal, FiTrash2, FiX } from 'react-icons/fi';
import { Panel } from 'react-resizable-panels';
import { twJoin, twMerge } from 'tailwind-merge';
import { tv } from 'tailwind-variants';

import { LogLevel, LogService, LogStreamResponseSchema } from '@the-dev-tools/spec/log/v1/log_pb';
import { Button, ButtonAsLink } from '@the-dev-tools/ui/button';
import { ChevronSolidDownIcon } from '@the-dev-tools/ui/icons';
import { PanelResizeHandle, panelResizeHandleStyles } from '@the-dev-tools/ui/resizable-panel';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TreeItemRoot, TreeItemWrapper } from '@the-dev-tools/ui/tree';

import type { WorkspaceRouteSearch } from './workspace/layout';

import { makeReferenceTreeId, ReferenceTreeItemView } from './reference';

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceIdCan');

export const useLogsQuery = () => {
  const { transport } = workspaceRoute.useRouteContext();

  const { logStream } = useMemo(() => createClient(LogService, transport), [transport]);

  const queryKey = useMemo(
    (): ConnectQueryKey<typeof LogStreamResponseSchema> =>
      createConnectQueryKey({
        cardinality: 'infinite',
        schema: { ...LogService.method.logStream, methodKind: 'unary' },
        transport,
      }),
    [transport],
  );

  const query = useQuery({
    queryFn: streamedQuery({
      maxChunks: 100,
      queryFn: () => logStream({}),
      refetchMode: 'append',
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
  const { showLogs } = workspaceRoute.useSearch();
  const { queryClient } = workspaceRoute.useRouteContext();

  const { data: logs, queryKey } = useLogsQuery();

  const separator = <div className={tw`h-3.5 w-px bg-slate-200`} />;

  const bar = (
    <div className={twMerge(tw`flex items-center gap-2 bg-slate-50 px-2 py-1`, showLogs && tw`bg-white`)}>
      <ButtonAsLink
        className={tw`px-2 py-1 text-xs leading-4 tracking-tight text-slate-800`}
        href={{
          search: (_: Partial<WorkspaceRouteSearch>) =>
            ({ ..._, showLogs: showLogs ? undefined : true }) satisfies Partial<WorkspaceRouteSearch>,
          to: '.',
        }}
        variant='ghost'
      >
        <FiTerminal className={tw`size-3`} />
        <span>Logs</span>
      </ButtonAsLink>

      <div className={tw`flex-1`} />

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

          <ButtonAsLink
            className={tw`p-0.5`}
            href={{
              search: (_: Partial<WorkspaceRouteSearch>) =>
                ({ ..._, showLogs: undefined }) satisfies Partial<WorkspaceRouteSearch>,
              to: '.',
            }}
            variant='ghost'
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
        <Panel>
          <div className={tw`flex size-full flex-col-reverse overflow-auto`}>
            <div>
              {logs?.map((_) => {
                const ulid = Ulid.construct(_.logId);
                return (
                  <AriaTree aria-label={_.value} key={ulid.toCanonical()}>
                    <TreeItemRoot textValue={_.value}>
                      <AriaTreeItemContent>
                        {({ isExpanded }) => (
                          <TreeItemWrapper className={tw`flex-wrap gap-1`} level={0}>
                            <Button className={tw`p-1`} slot='chevron' variant='ghost'>
                              <ChevronSolidDownIcon
                                className={twJoin(
                                  tw`size-3 text-slate-500 transition-transform`,
                                  !isExpanded ? tw`rotate-0` : tw`rotate-90`,
                                )}
                              />
                            </Button>

                            <div className={logTextStyles({ level: _.level })}>
                              {ulid.time.toLocaleTimeString()}: {_.value}
                            </div>
                          </TreeItemWrapper>
                        )}
                      </AriaTreeItemContent>

                      <AriaCollection items={_.references}>
                        {(_) => (
                          <ReferenceTreeItemView
                            id={makeReferenceTreeId([_.key!], _.value)}
                            parentKeys={[]}
                            reference={_}
                          />
                        )}
                      </AriaCollection>
                    </TreeItemRoot>
                  </AriaTree>
                );
              })}
            </div>
          </div>
        </Panel>
      )}
    </>
  );
};
