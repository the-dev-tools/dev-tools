import { useLiveQuery } from '@tanstack/react-db';
import { useEffect } from 'react';
import { HttpCollectionSchema, HttpDeltaCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { MethodBadge } from '@the-dev-tools/ui/method-badge';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useDeltaState } from '~/features/delta';
import { useApiCollection } from '~/shared/api';
import { eqStruct } from '~/shared/lib';
import { routes } from '~/shared/routes';
import { useCloseTab } from '~/widgets/tabs';

export interface HttpTabProps {
  deltaHttpId?: Uint8Array;
  httpId: Uint8Array;
}

export const httpTabId = ({ deltaHttpId, httpId }: HttpTabProps) =>
  JSON.stringify({ deltaHttpId, httpId, route: routes.dashboard.workspace.http.route.id });

export const HttpTab = ({ deltaHttpId, httpId }: HttpTabProps) => {
  const closeTab = useCloseTab();

  const httpCollection = useApiCollection(HttpCollectionSchema);

  const httpExists =
    useLiveQuery(
      (_) => _.from({ item: httpCollection }).where(eqStruct({ httpId })).findOne(),
      [httpCollection, httpId],
    ).data !== undefined;

  useEffect(() => {
    if (!httpExists) void closeTab(httpTabId({ httpId }));
  }, [httpExists, httpId, closeTab]);

  const deltaCollection = useApiCollection(HttpDeltaCollectionSchema);

  const deltaExists =
    useLiveQuery(
      (_) => _.from({ item: deltaCollection }).where(eqStruct({ deltaHttpId })).findOne(),
      [deltaCollection, deltaHttpId],
    ).data !== undefined;

  useEffect(() => {
    if (deltaHttpId && !deltaExists) void closeTab(httpTabId({ deltaHttpId, httpId }));
  }, [deltaExists, deltaHttpId, httpId, closeTab]);

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
