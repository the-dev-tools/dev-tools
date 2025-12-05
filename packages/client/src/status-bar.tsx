import { useLiveQuery } from '@tanstack/react-db';
import { Ulid } from 'id128';
import { Tree as AriaTree } from 'react-aria-components';
import { FiTerminal, FiTrash2, FiX } from 'react-icons/fi';
import { Panel } from 'react-resizable-panels';
import { twMerge } from 'tailwind-merge';
import { tv } from 'tailwind-variants';
import { LogLevel } from '@the-dev-tools/spec/buf/api/log/v1/log_pb';
import { LogCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/log';
import { Button, ButtonAsLink } from '@the-dev-tools/ui/button';
import { JsonTreeItem, jsonTreeItemProps } from '@the-dev-tools/ui/json-tree';
import { PanelResizeHandle, panelResizeHandleStyles } from '@the-dev-tools/ui/resizable-panel';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TreeItem } from '@the-dev-tools/ui/tree';
import { useApiCollection } from '~api';
import { workspaceRouteApi } from '~routes';

const logTextStyles = tv({
  base: tw`font-mono text-sm`,
  variants: {
    level: {
      [LogLevel.ERROR]: tw`text-red-600`,
      [LogLevel.INFO]: tw`text-slate-800`,
      [LogLevel.UNSPECIFIED]: tw`text-slate-800`,
      [LogLevel.WARNING]: tw`text-yellow-600`,
    } satisfies Record<LogLevel, string>,
  },
});

export const StatusBar = () => {
  const logCollection = useApiCollection(LogCollectionSchema);
  const { data: logs } = useLiveQuery((_) => _.from({ log: logCollection }), [logCollection]);

  const { showLogs } = workspaceRouteApi.useSearch();

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
            onPress={() => {
              const state = logCollection.utils.state();
              state.begin();
              state.truncate();
              state.commit();
            }}
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
            <AriaTree aria-label='Logs' items={logs}>
              {(_) => {
                const ulid = Ulid.construct(_.logId);
                const id = ulid.toCanonical();
                return (
                  <TreeItem
                    id={id}
                    item={(_) => <JsonTreeItem {..._} id={`${id}.${_.id ?? 'root'}`} />}
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
