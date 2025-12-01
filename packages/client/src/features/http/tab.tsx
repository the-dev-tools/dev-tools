import { useLiveQuery } from '@tanstack/react-db';
import { useEffect } from 'react';
import { HttpCollectionSchema, HttpDeltaCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { MethodBadge } from '@the-dev-tools/ui/method-badge';
import { useRemoveTab } from '@the-dev-tools/ui/router';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/api';
import { httpRouteApi, workspaceRouteApi } from '~/routes';
import { useDeltaState } from '~/utils/delta';
import { eqStruct } from '~/utils/tanstack-db';

export interface HttpTabProps {
  deltaHttpId?: Uint8Array;
  httpId: Uint8Array;
}

export const httpTabId = ({ deltaHttpId, httpId }: HttpTabProps) =>
  JSON.stringify({ deltaHttpId, httpId, route: httpRouteApi.id });

export const HttpTab = ({ deltaHttpId, httpId }: HttpTabProps) => {
  const context = workspaceRouteApi.useRouteContext();

  const removeTab = useRemoveTab();

  const httpCollection = useApiCollection(HttpCollectionSchema);

  const httpExists =
    useLiveQuery(
      (_) => _.from({ item: httpCollection }).where(eqStruct({ httpId })).findOne(),
      [httpCollection, httpId],
    ).data !== undefined;

  useEffect(() => {
    if (!httpExists) void removeTab({ ...context, id: httpTabId({ httpId }) });
  }, [context, httpExists, httpId, removeTab]);

  const deltaCollection = useApiCollection(HttpDeltaCollectionSchema);

  const deltaExists =
    useLiveQuery(
      (_) => _.from({ item: deltaCollection }).where(eqStruct({ deltaHttpId })).findOne(),
      [deltaCollection, deltaHttpId],
    ).data !== undefined;

  useEffect(() => {
    if (deltaHttpId && !deltaExists) void removeTab({ ...context, id: httpTabId({ deltaHttpId, httpId }) });
  }, [context, deltaExists, deltaHttpId, httpId, removeTab]);

  const deltaOptions = {
    deltaId: deltaHttpId,
    deltaSchema: HttpDeltaCollectionSchema,
    isDelta: deltaHttpId !== undefined,
    originId: httpId,
    originSchema: HttpCollectionSchema,
  };

  const [method] = useDeltaState({ ...deltaOptions, valueKey: 'method' });
  const [name] = useDeltaState({ ...deltaOptions, valueKey: 'name' });

  return (
    <>
      {method && <MethodBadge method={method} />}
      <span className={tw`min-w-0 flex-1 truncate`}>{name}</span>
    </>
  );
};
