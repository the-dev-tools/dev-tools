import { HttpCollectionSchema, HttpDeltaCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { MethodBadge } from '@the-dev-tools/ui/method-badge';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { httpRouteApi } from '~/routes';
import { useDeltaState } from '~utils/delta';

export interface HttpTabProps {
  deltaHttpId?: Uint8Array;
  httpId: Uint8Array;
}

export const httpTabId = ({ deltaHttpId, httpId }: HttpTabProps) =>
  JSON.stringify({ deltaHttpId, httpId, route: httpRouteApi.id });

export const HttpTab = ({ deltaHttpId, httpId }: HttpTabProps) => {
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
