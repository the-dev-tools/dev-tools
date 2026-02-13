import { eq, useLiveQuery } from '@tanstack/react-db';
import { useEffect } from 'react';
import { FlowCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { FlowsIcon } from '@the-dev-tools/ui/icons';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/shared/api';
import { pick } from '~/shared/lib';
import { routes } from '~/shared/routes';
import { useCloseTab } from '~/widgets/tabs';

interface FlowTabProps {
  flowId: Uint8Array;
}

export const flowTabId = (flowId: Uint8Array) =>
  JSON.stringify({ flowId, route: routes.dashboard.workspace.flow.route.id });

export const FlowTab = ({ flowId }: FlowTabProps) => {
  const closeTab = useCloseTab();

  const collection = useApiCollection(FlowCollectionSchema);

  const flow = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where((_) => eq(_.item.flowId, flowId))
        .select((_) => pick(_.item, 'name'))
        .findOne(),
    [collection, flowId],
  ).data;

  const flowExists = flow !== undefined;

  useEffect(() => {
    if (!flowExists) void closeTab(flowTabId(flowId));
  }, [flowExists, flowId, closeTab]);

  return (
    <>
      <FlowsIcon className={tw`size-5 shrink-0 text-on-neutral-low`} />
      <span className={tw`min-w-0 flex-1 truncate`}>{flow?.name}</span>
    </>
  );
};
