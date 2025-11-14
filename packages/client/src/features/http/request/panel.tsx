import { count, eq, useLiveQuery } from '@tanstack/react-db';
import { Suspense } from 'react';
import { Tab, TabList, TabPanel, Tabs } from 'react-aria-components';
import { twMerge } from 'tailwind-merge';
import { HttpBodyKind } from '@the-dev-tools/spec/api/http/v1/http_pb';
import {
  HttpAssertCollectionSchema,
  HttpBodyFormDataCollectionSchema,
  HttpBodyUrlEncodedCollectionSchema,
  HttpCollectionSchema,
  HttpHeaderCollectionSchema,
  HttpSearchParamCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { Spinner } from '@the-dev-tools/ui/spinner';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/api-new';
import { pick } from '~/utils/tanstack-db';
import { AssertPanel } from './assert';
import { BodyPanel } from './body/panel';
import { HeaderTable } from './header';
import { SearchParamTable } from './search-param';

export interface HttpRequestPanelProps {
  className?: string;
  httpId: Uint8Array;
  isReadOnly?: boolean;
}

export const HttpRequestPanel = ({ className, httpId, isReadOnly = false }: HttpRequestPanelProps) => {
  const searchParamCollection = useApiCollection(HttpSearchParamCollectionSchema);

  const { data: { searchParamCount = 0 } = {} } = useLiveQuery(
    (_) =>
      _.from({ item: searchParamCollection })
        .where((_) => eq(_.item.httpId, httpId))
        .select((_) => ({ searchParamCount: count(_.item.httpId) }))
        .findOne(),
    [httpId, searchParamCollection],
  );

  const headerCollection = useApiCollection(HttpHeaderCollectionSchema);

  const { data: { headerCount = 0 } = {} } = useLiveQuery(
    (_) =>
      _.from({ item: headerCollection })
        .where((_) => eq(_.item.httpId, httpId))
        .select((_) => ({ headerCount: count(_.item.httpId) }))
        .findOne(),
    [headerCollection, httpId],
  );

  const httpCollection = useApiCollection(HttpCollectionSchema);

  const { data: { bodyKind } = {} } = useLiveQuery(
    (_) =>
      _.from({ item: httpCollection })
        .where((_) => eq(_.item.httpId, httpId))
        .select((_) => pick(_.item, 'bodyKind'))
        .findOne(),
    [httpCollection, httpId],
  );

  const bodyFormDataCollection = useApiCollection(HttpBodyFormDataCollectionSchema);

  const { data: { bodyFormDataCount = 0 } = {} } = useLiveQuery(
    (_) =>
      _.from({ item: bodyFormDataCollection })
        .where((_) => eq(_.item.httpId, httpId))
        .select((_) => ({ bodyFormDataCount: count(_.item.httpId) }))
        .findOne(),
    [bodyFormDataCollection, httpId],
  );

  const bodyUrlEncodedCollection = useApiCollection(HttpBodyUrlEncodedCollectionSchema);

  const { data: { bodyUrlEncodedCount = 0 } = {} } = useLiveQuery(
    (_) =>
      _.from({ item: bodyUrlEncodedCollection })
        .where((_) => eq(_.item.httpId, httpId))
        .select((_) => ({ bodyUrlEncodedCount: count(_.item.httpId) }))
        .findOne(),
    [bodyUrlEncodedCollection, httpId],
  );

  const assertCollection = useApiCollection(HttpAssertCollectionSchema);

  const { data: { assertCount = 0 } = {} } = useLiveQuery(
    (_) =>
      _.from({ item: assertCollection })
        .where((_) => eq(_.item.httpId, httpId))
        .select((_) => ({ assertCount: count(_.item.httpId) }))
        .findOne(),
    [assertCollection, httpId],
  );

  return (
    <Tabs className={twMerge(tw`flex flex-1 flex-col gap-6 overflow-auto p-6 pt-4`, className)}>
      <TabList className={tw`flex gap-3 border-b border-slate-200`}>
        <Tab
          className={({ isSelected }) =>
            twMerge(
              tw`
                -mb-px cursor-pointer border-b-2 border-transparent py-1.5 text-md leading-5 font-medium tracking-tight
                text-slate-500 transition-colors
              `,
              isSelected && tw`border-b-violet-700 text-slate-800`,
            )
          }
          id='params'
        >
          Search Params
          {searchParamCount > 0 && <span className={tw`text-xs text-green-600`}> ({searchParamCount})</span>}
        </Tab>

        <Tab
          className={({ isSelected }) =>
            twMerge(
              tw`
                -mb-px cursor-pointer border-b-2 border-transparent py-1.5 text-md leading-5 font-medium tracking-tight
                text-slate-500 transition-colors
              `,
              isSelected && tw`border-b-violet-700 text-slate-800`,
            )
          }
          id='headers'
        >
          Headers
          {headerCount > 0 && <span className={tw`text-xs text-green-600`}> ({headerCount})</span>}
        </Tab>

        <Tab
          className={({ isSelected }) =>
            twMerge(
              tw`
                -mb-px cursor-pointer border-b-2 border-transparent py-1.5 text-md leading-5 font-medium tracking-tight
                text-slate-500 transition-colors
              `,
              isSelected && tw`border-b-violet-700 text-slate-800`,
            )
          }
          id='body'
        >
          Body
          {bodyKind === HttpBodyKind.FORM_DATA && bodyFormDataCount > 0 && (
            <span className={tw`text-xs text-green-600`}> ({bodyFormDataCount})</span>
          )}
          {bodyKind === HttpBodyKind.URL_ENCODED && bodyUrlEncodedCount > 0 && (
            <span className={tw`text-xs text-green-600`}> ({bodyUrlEncodedCount})</span>
          )}
        </Tab>

        <Tab
          className={({ isSelected }) =>
            twMerge(
              tw`
                -mb-px cursor-pointer border-b-2 border-transparent py-1.5 text-md leading-5 font-medium tracking-tight
                text-slate-500 transition-colors
              `,
              isSelected && tw`border-b-violet-700 text-slate-800`,
            )
          }
          id='assertions'
        >
          Assertion
          {assertCount > 0 && <span className={tw`text-xs text-green-600`}> ({assertCount})</span>}
        </Tab>
      </TabList>

      <Suspense
        fallback={
          <div className={tw`flex h-full items-center justify-center`}>
            <Spinner size='lg' />
          </div>
        }
      >
        <TabPanel id='params'>
          <SearchParamTable httpId={httpId} isReadOnly={isReadOnly} />
        </TabPanel>

        <TabPanel id='headers'>
          <HeaderTable httpId={httpId} isReadOnly={isReadOnly} />
        </TabPanel>

        <TabPanel className={tw`h-full`} id='body'>
          <BodyPanel httpId={httpId} isReadOnly={isReadOnly} />
        </TabPanel>

        <TabPanel id='assertions'>
          <AssertPanel httpId={httpId} isReadOnly={isReadOnly} />
        </TabPanel>
      </Suspense>
    </Tabs>
  );
};
