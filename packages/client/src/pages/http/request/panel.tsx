import { count, eq, or, useLiveQuery } from '@tanstack/react-db';
import { Suspense } from 'react';
import { Tab, TabList, TabPanel, Tabs } from 'react-aria-components';
import { twMerge } from 'tailwind-merge';
import { HttpBodyKind } from '@the-dev-tools/spec/buf/api/http/v1/http_pb';
import {
  HttpAssertCollectionSchema,
  HttpBodyFormDataCollectionSchema,
  HttpBodyUrlEncodedCollectionSchema,
  HttpCollectionSchema,
  HttpDeltaCollectionSchema,
  HttpHeaderCollectionSchema,
  HttpSearchParamCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { Spinner } from '@the-dev-tools/ui/spinner';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useDeltaState } from '~/features/delta';
import { useApiCollection } from '~/shared/api';
import { AssertTable } from './assert';
import { BodyPanel } from './body/panel';
import { HeaderTable } from './header';
import { SearchParamTable } from './search-param';

export interface HttpRequestPanelProps {
  className?: string;
  deltaHttpId: Uint8Array | undefined;
  hideDescription?: boolean;
  httpId: Uint8Array;
  isReadOnly?: boolean;
}

export const HttpRequestPanel = ({
  className,
  deltaHttpId,
  hideDescription = false,
  httpId,
  isReadOnly = false,
}: HttpRequestPanelProps) => {
  const searchParamCollection = useApiCollection(HttpSearchParamCollectionSchema);

  const { searchParamCount = 0 } =
    useLiveQuery(
      (_) =>
        _.from({ item: searchParamCollection })
          .where((_) => or(eq(_.item.httpId, httpId), eq(_.item.httpId, deltaHttpId)))
          .select((_) => ({ searchParamCount: count(_.item.httpId) }))
          .findOne(),
      [deltaHttpId, httpId, searchParamCollection],
    ).data ?? {};

  const headerCollection = useApiCollection(HttpHeaderCollectionSchema);

  const { headerCount = 0 } =
    useLiveQuery(
      (_) =>
        _.from({ item: headerCollection })
          .where((_) => or(eq(_.item.httpId, httpId), eq(_.item.httpId, deltaHttpId)))
          .select((_) => ({ headerCount: count(_.item.httpId) }))
          .findOne(),
      [deltaHttpId, headerCollection, httpId],
    ).data ?? {};

  const [bodyKind] = useDeltaState({
    deltaId: deltaHttpId,
    deltaSchema: HttpDeltaCollectionSchema,
    isDelta: deltaHttpId !== undefined,
    originId: httpId,
    originSchema: HttpCollectionSchema,
    valueKey: 'bodyKind',
  });

  const bodyFormDataCollection = useApiCollection(HttpBodyFormDataCollectionSchema);

  const { bodyFormDataCount = 0 } =
    useLiveQuery(
      (_) =>
        _.from({ item: bodyFormDataCollection })
          .where((_) => or(eq(_.item.httpId, httpId), eq(_.item.httpId, deltaHttpId)))
          .select((_) => ({ bodyFormDataCount: count(_.item.httpId) }))
          .findOne(),
      [bodyFormDataCollection, deltaHttpId, httpId],
    ).data ?? {};

  const bodyUrlEncodedCollection = useApiCollection(HttpBodyUrlEncodedCollectionSchema);

  const { bodyUrlEncodedCount = 0 } =
    useLiveQuery(
      (_) =>
        _.from({ item: bodyUrlEncodedCollection })
          .where((_) => or(eq(_.item.httpId, httpId), eq(_.item.httpId, deltaHttpId)))
          .select((_) => ({ bodyUrlEncodedCount: count(_.item.httpId) }))
          .findOne(),
      [bodyUrlEncodedCollection, deltaHttpId, httpId],
    ).data ?? {};

  const assertCollection = useApiCollection(HttpAssertCollectionSchema);

  const { assertCount = 0 } =
    useLiveQuery(
      (_) =>
        _.from({ item: assertCollection })
          .where((_) => or(eq(_.item.httpId, httpId), eq(_.item.httpId, deltaHttpId)))
          .select((_) => ({ assertCount: count(_.item.httpId) }))
          .findOne(),
      [assertCollection, deltaHttpId, httpId],
    ).data ?? {};

  return (
    <Tabs className={twMerge(tw`flex flex-1 flex-col gap-6 overflow-auto p-6 pt-4`, className)}>
      <TabList className={tw`flex gap-3 border-b border-border`}>
        <Tab
          className={({ isSelected }) =>
            twMerge(
              tw`
                -mb-px cursor-pointer border-b-2 border-transparent py-1.5 text-md leading-5 font-medium tracking-tight
                text-fg-muted transition-colors
              `,
              isSelected && tw`border-b-accent-border text-fg`,
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
                text-fg-muted transition-colors
              `,
              isSelected && tw`border-b-accent-border text-fg`,
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
                text-fg-muted transition-colors
              `,
              isSelected && tw`border-b-accent-border text-fg`,
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
                text-fg-muted transition-colors
              `,
              isSelected && tw`border-b-accent-border text-fg`,
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
          <SearchParamTable
            deltaHttpId={deltaHttpId}
            hideDescription={hideDescription}
            httpId={httpId}
            isReadOnly={isReadOnly}
          />
        </TabPanel>

        <TabPanel id='headers'>
          <HeaderTable
            deltaHttpId={deltaHttpId}
            hideDescription={hideDescription}
            httpId={httpId}
            isReadOnly={isReadOnly}
          />
        </TabPanel>

        <TabPanel className={tw`h-full`} id='body'>
          <BodyPanel
            deltaHttpId={deltaHttpId}
            hideDescription={hideDescription}
            httpId={httpId}
            isReadOnly={isReadOnly}
          />
        </TabPanel>

        <TabPanel id='assertions'>
          <AssertTable deltaHttpId={deltaHttpId} httpId={httpId} isReadOnly={isReadOnly} />
        </TabPanel>
      </Suspense>
    </Tabs>
  );
};
