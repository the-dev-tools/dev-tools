import { eq, useLiveQuery } from '@tanstack/react-db';
import { Fragment } from 'react/jsx-runtime';
import { twJoin } from 'tailwind-merge';
import { HttpResponseAssertCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/api';
import { pick } from '~/utils/tanstack-db';

export interface AssertTableProps {
  httpResponseId: Uint8Array;
}

export const AssertTable = ({ httpResponseId }: AssertTableProps) => {
  const collection = useApiCollection(HttpResponseAssertCollectionSchema);

  const { data: items } = useLiveQuery(
    (_) =>
      _.from({ item: collection })
        .where((_) => eq(_.item.httpResponseId, httpResponseId))
        .select((_) => pick(_.item, 'httpResponseAssertId', 'value', 'success')),
    [collection, httpResponseId],
  );

  return (
    <div className={tw`grid grid-cols-[auto_1fr] items-center gap-2 text-sm`}>
      {items.map((_) => (
        <Fragment key={collection.utils.getKey(_)}>
          <div
            className={twJoin(
              tw`rounded-sm px-2 py-1 text-center font-light text-white uppercase`,
              _.success ? tw`bg-green-600` : tw`bg-red-600`,
            )}
          >
            {_.success ? 'Pass' : 'Fail'}
          </div>

          <span>{_.value}</span>
        </Fragment>
      ))}
    </div>
  );
};
