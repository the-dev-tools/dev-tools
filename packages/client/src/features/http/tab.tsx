import { eq, useLiveQuery } from '@tanstack/react-db';
import { Option, pipe } from 'effect';
import { HttpCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { MethodBadge } from '@the-dev-tools/ui/method-badge';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/api-new';
import { httpRouteApi } from '~/routes';
import { pick } from '~/utils/tanstack-db';

export interface HttpTabProps {
  httpId: Uint8Array;
}

export const httpTabId = ({ httpId }: HttpTabProps) => JSON.stringify({ httpId, route: httpRouteApi.id });

export const HttpTab = ({ httpId }: HttpTabProps) => {
  const httpCollection = useApiCollection(HttpCollectionSchema);

  const { method, name } = pipe(
    useLiveQuery(
      (_) =>
        _.from({ item: httpCollection })
          .where((_) => eq(_.item.httpId, httpId))
          .select((_) => pick(_.item, 'name', 'method'))
          .findOne(),
      [httpCollection, httpId],
    ),
    (_) => Option.fromNullable(_.data),
    Option.getOrThrow,
  );

  return (
    <>
      {method && <MethodBadge method={method} />}
      <span className={tw`min-w-0 flex-1 truncate`}>{name}</span>
    </>
  );
};
