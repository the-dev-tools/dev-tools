import { eq, useLiveQuery } from '@tanstack/react-db';
import { useEffect } from 'react';
import { FlowCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { FlowsIcon } from '@the-dev-tools/ui/icons';
import { useRemoveTab } from '@the-dev-tools/ui/router';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/api-new';
import { flowLayoutRouteApi, workspaceRouteApi } from '~/routes';
import { pick } from '~/utils/tanstack-db';

interface FlowTabProps {
  flowId: Uint8Array;
}

export const flowTabId = (flowId: Uint8Array) => JSON.stringify({ flowId, route: flowLayoutRouteApi.id });

export const FlowTab = ({ flowId }: FlowTabProps) => {
  const context = workspaceRouteApi.useRouteContext();

  const removeTab = useRemoveTab();

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
    if (!flowExists) void removeTab({ ...context, id: flowTabId(flowId) });
  }, [context, flowExists, flowId, removeTab]);

  return (
    <>
      <FlowsIcon className={tw`size-5 shrink-0 text-slate-500`} />
      <span className={tw`min-w-0 flex-1 truncate`}>{flow?.name}</span>
    </>
  );
};
