import { count, eq, useLiveQuery } from '@tanstack/react-db';
import { Duration, Option, pipe } from 'effect';
import { Suspense } from 'react';
import { Tab, TabList, TabPanel, Tabs } from 'react-aria-components';
import { twJoin, twMerge } from 'tailwind-merge';
import {
  HttpResponseAssertCollectionSchema,
  HttpResponseCollectionSchema,
  HttpResponseHeaderCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { Separator } from '@the-dev-tools/ui/separator';
import { Spinner } from '@the-dev-tools/ui/spinner';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { formatSize } from '@the-dev-tools/ui/utils';
import { useApiCollection } from '~/api';
import { pick } from '~/utils/tanstack-db';
import { AssertTable } from './assert';
import { BodyPanel } from './body';
import { HeaderTable } from './header';

export interface ResponsePanelProps {
  className?: string;
  fullWidth?: boolean;
  httpResponseId: Uint8Array;
}

export const ResponsePanel = ({ className, fullWidth = false, httpResponseId }: ResponsePanelProps) => {
  const responseCollection = useApiCollection(HttpResponseCollectionSchema);

  const { duration, size, status } = pipe(
    useLiveQuery(
      (_) =>
        _.from({ item: responseCollection })
          .where((_) => eq(_.item.httpResponseId, httpResponseId))
          .select((_) => pick(_.item, 'duration', 'size', 'status'))
          .findOne(),
      [responseCollection, httpResponseId],
    ),
    (_) => Option.fromNullable(_.data),
    Option.getOrThrow,
  );

  const headerCollection = useApiCollection(HttpResponseHeaderCollectionSchema);

  const { data: { headerCount = 0 } = {} } = useLiveQuery(
    (_) =>
      _.from({ item: headerCollection })
        .where((_) => eq(_.item.httpResponseId, httpResponseId))
        .select((_) => ({ headerCount: count(_.item.httpResponseHeaderId) }))
        .findOne(),
    [headerCollection, httpResponseId],
  );

  const assertCollection = useApiCollection(HttpResponseAssertCollectionSchema);

  const { data: { assertCount = 0 } = {} } = useLiveQuery(
    (_) =>
      _.from({ item: assertCollection })
        .where((_) => eq(_.item.httpResponseId, httpResponseId))
        .select((_) => ({ assertCount: count(_.item.httpResponseAssertId) }))
        .findOne(),
    [assertCollection, httpResponseId],
  );

  return (
    <Tabs className={twMerge(tw`flex h-full flex-col pb-4`, className)}>
      <div className={twMerge(tw`flex items-center gap-3 border-b border-slate-200 text-md`, fullWidth && tw`px-4`)}>
        <TabList className={tw`flex items-center gap-3`}>
          <Tab
            className={({ isSelected }) =>
              twMerge(
                tw`
                  -mb-px cursor-pointer border-b-2 border-transparent py-2 text-md leading-5 font-medium tracking-tight
                  text-slate-500 transition-colors
                `,
                isSelected && tw`border-b-violet-700 text-slate-800`,
              )
            }
            id='body'
          >
            Body
          </Tab>

          <Tab
            className={({ isSelected }) =>
              twMerge(
                tw`
                  -mb-px cursor-pointer border-b-2 border-transparent py-2 text-md leading-5 font-medium tracking-tight
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
                  -mb-px cursor-pointer border-b-2 border-transparent py-2 text-md leading-5 font-medium tracking-tight
                  text-slate-500 transition-colors
                `,
                isSelected && tw`border-b-violet-700 text-slate-800`,
              )
            }
            id='assertions'
          >
            Assertion Results
            {assertCount > 0 && <span className={tw`text-xs text-green-600`}> ({assertCount})</span>}
          </Tab>
        </TabList>

        <div className={tw`flex-1`} />

        <div className={tw`flex items-center gap-1 text-xs leading-5 font-medium tracking-tight text-slate-800`}>
          <div className={tw`flex gap-1 p-2`}>
            <span>Status:</span>
            <span className={tw`text-green-600`}>{status}</span>
          </div>

          <Separator className={tw`h-4`} orientation='vertical' />

          <div className={tw`flex gap-1 p-2`}>
            <span>Time:</span>
            <span className={tw`text-green-600`}>{pipe(duration, Duration.millis, Duration.format)}</span>
          </div>

          <Separator className={tw`h-4`} orientation='vertical' />

          <div className={tw`flex gap-1 p-2`}>
            <span>Size:</span>
            <span>{formatSize(size)}</span>
          </div>
        </div>
      </div>

      <div className={twJoin(tw`flex-1 overflow-auto pt-4`, fullWidth && tw`px-4`)}>
        <Suspense
          fallback={
            <div className={tw`flex h-full items-center justify-center`}>
              <Spinner size='lg' />
            </div>
          }
        >
          <TabPanel className={twJoin(tw`flex h-full flex-col gap-4`)} id='body'>
            <BodyPanel httpResponseId={httpResponseId} />
          </TabPanel>

          <TabPanel id='headers'>
            <HeaderTable httpResponseId={httpResponseId} />
          </TabPanel>

          <TabPanel id='assertions'>
            <AssertTable httpResponseId={httpResponseId} />
          </TabPanel>
        </Suspense>
      </div>
    </Tabs>
  );
};
