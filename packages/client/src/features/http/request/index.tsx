import { MessageInitShape } from '@bufbuild/protobuf';
import { count, eq, useLiveQuery } from '@tanstack/react-db';
import { Array, flow, MutableHashSet, Option, pipe, Record, String, Struct } from 'effect';
import { Ulid } from 'id128';
import { Suspense, useTransition } from 'react';
import { Button as AriaButton, DialogTrigger, MenuTrigger, Tab, TabList, TabPanel, Tabs } from 'react-aria-components';
import { useForm } from 'react-hook-form';
import { FiClock, FiMoreHorizontal } from 'react-icons/fi';
import { twMerge } from 'tailwind-merge';
import {
  HttpBodyKind,
  HttpMethodSchema,
  HttpSearchParamInsertSchema,
  HttpSearchParamUpdateSchema,
  HttpService,
} from '@the-dev-tools/spec/api/http/v1/http_pb';
import {
  HttpAssertCollectionSchema,
  HttpBodyFormDataCollectionSchema,
  HttpBodyUrlEncodedCollectionSchema,
  HttpCollectionSchema,
  HttpHeaderCollectionSchema,
  HttpSearchParamCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { Button } from '@the-dev-tools/ui/button';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { MethodBadge } from '@the-dev-tools/ui/method-badge';
import { Select, SelectItem } from '@the-dev-tools/ui/select';
import { Separator } from '@the-dev-tools/ui/separator';
import { Spinner } from '@the-dev-tools/ui/spinner';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { Connect, Protobuf, useApiCollection } from '~/api-new';
import { ReferenceFieldRHF } from '~/reference';
import { rootRouteApi } from '~/routes';
import { pick, queryCollection } from '~/utils/tanstack-db';
import { AssertPanel } from './assert';
import { BodyPanel } from './body/panel';
import { HeaderTable } from './header';
import { SearchParamTable } from './search-param';

export interface HttpRequestProps {
  className?: string;
  httpId: Uint8Array;
}

export const HttpRequest = ({ className, httpId }: HttpRequestProps) => {
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
          <SearchParamTable httpId={httpId} />
        </TabPanel>

        <TabPanel id='headers'>
          <HeaderTable httpId={httpId} />
        </TabPanel>

        <TabPanel className={tw`h-full`} id='body'>
          <BodyPanel httpId={httpId} />
        </TabPanel>

        <TabPanel id='assertions'>
          <AssertPanel httpId={httpId} />
        </TabPanel>
      </Suspense>
    </Tabs>
  );
};

export interface HttpTopBarProps {
  httpId: Uint8Array;
}

export const HttpTopBar = ({ httpId }: HttpTopBarProps) => {
  const { transport } = rootRouteApi.useRouteContext();

  const httpCollection = useApiCollection(HttpCollectionSchema);

  const { name } = pipe(
    useLiveQuery(
      (_) =>
        _.from({ item: httpCollection })
          .where((_) => eq(_.item.httpId, httpId))
          .select((_) => pick(_.item, 'name'))
          .findOne(),
      [httpCollection, httpId],
    ),
    (_) => Option.fromNullable(_.data),
    Option.getOrThrow,
  );

  const searchParamCollection = useApiCollection(HttpSearchParamCollectionSchema);

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => httpCollection.utils.update({ httpId, name: _ }),
    value: name,
  });

  const [isSending, startTransition] = useTransition();

  return (
    <>
      <div className='flex items-center gap-2 border-b border-slate-200 px-4 py-2.5'>
        <div
          className={tw`
            flex min-w-0 flex-1 gap-1 text-md leading-5 font-medium tracking-tight text-slate-400 select-none
          `}
        >
          {/* {example.breadcrumbs.map((_, index) => {
            // TODO: add links to breadcrumbs
            const key = enumToString(ExampleBreadcrumbKindSchema, 'EXAMPLE_BREADCRUMB_KIND', _.kind);
            const name = _[key]?.name;
            return (
              <Fragment key={`${index} ${name}`}>
                <span>{name}</span>
                <span>/</span>
              </Fragment>
            );
          })} */}

          {isEditing ? (
            <TextInputField
              aria-label='Example name'
              inputClassName={tw`-my-1 py-1 leading-none text-slate-800`}
              {...textFieldProps}
            />
          ) : (
            <AriaButton
              className={tw`max-w-full cursor-text truncate text-slate-800`}
              onContextMenu={onContextMenu}
              onPress={() => void edit()}
            >
              {name}
            </AriaButton>
          )}
        </div>

        <DialogTrigger>
          <Button className={tw`px-2 py-1 text-slate-800`} variant='ghost'>
            <FiClock className={tw`size-4 text-slate-500`} /> Response History
          </Button>

          {/* <HistoryModal endpointId={endpointId} exampleId={exampleId} /> */}
        </DialogTrigger>

        <MenuTrigger {...menuTriggerProps}>
          <Button className={tw`p-1`} variant='ghost'>
            <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
          </Button>

          <Menu {...menuProps}>
            <MenuItem onAction={() => void edit()}>Rename</MenuItem>

            <MenuItem onAction={() => void httpCollection.utils.delete({ httpId })} variant='danger'>
              Delete
            </MenuItem>
          </Menu>
        </MenuTrigger>
      </div>

      <div className={tw`flex gap-3 p-6 pb-0`}>
        <HttpUrl httpId={httpId} />

        <Button
          className={tw`px-6`}
          isPending={isSending}
          onPress={() =>
            void startTransition(async () => {
              const httpTransactions = Array.fromIterable(httpCollection._state.transactions.values());
              const searchParamTransactions = Array.fromIterable(searchParamCollection._state.transactions.values());

              await pipe(
                Array.appendAll(httpTransactions, searchParamTransactions),
                Array.map((_) => _.isPersisted.promise),
                (_) => Promise.all(_),
              );

              await Connect.request({ input: { httpId }, method: HttpService.method.httpRun, transport });
            })
          }
          variant='primary'
        >
          Send
        </Button>
      </div>
    </>
  );
};

export interface HttpUrlProps {
  httpId: Uint8Array;
}

export const HttpUrl = ({ httpId }: HttpUrlProps) => {
  const httpCollection = useApiCollection(HttpCollectionSchema);

  const { method, url } = pipe(
    useLiveQuery(
      (_) =>
        _.from({ item: httpCollection })
          .where((_) => eq(_.item.httpId, httpId))
          .select((_) => pick(_.item, 'url', 'method'))
          .findOne(),
      [httpCollection, httpId],
    ),
    (_) => Option.fromNullable(_.data),
    Option.getOrThrow,
  );

  const searchParamCollection = useApiCollection(HttpSearchParamCollectionSchema);

  const { data: searchParams } = useLiveQuery(
    (_) =>
      _.from({ item: searchParamCollection })
        .where((_) => eq(_.item.httpId, httpId))
        .orderBy((_) => _.item.order)
        .select((_) => pick(_.item, 'httpSearchParamId', 'order', 'enabled', 'key', 'value')),
    [httpId, searchParamCollection],
  );

  const searchParamString = pipe(
    searchParams,
    Array.filterMap(
      flow(
        Option.liftPredicate((_) => _.enabled),
        Option.map((_) => `${_.key}=${_.value}`),
      ),
    ),
    Array.join('&'),
  );

  let urlString = url;
  if (searchParamString.length > 0) urlString += '?' + searchParamString;

  const form = useForm({ values: { urlString } });

  const submit = form.handleSubmit(async ({ urlString }) => {
    const { searchParamString, url } = pipe(
      urlString,
      String.indexOf('?'),
      Option.match({
        onNone: () => ({ searchParamString: '', url: urlString }),
        onSome: (separator) => ({
          searchParamString: urlString.slice(separator + 1),
          url: urlString.slice(0, separator),
        }),
      }),
    );

    httpCollection.utils.update({ httpId, url });

    const searchParamSet = pipe(
      searchParamString,
      Option.liftPredicate(String.isNonEmpty),
      Option.map(String.split('&')),
      Option.getOrElse(Array.empty),
      MutableHashSet.fromIterable,
    );

    pipe(
      Array.filterMap(searchParams, (_) => {
        const searchParamString = `${_.key}=${_.value}`;
        const enabled = MutableHashSet.has(searchParamSet, searchParamString);
        MutableHashSet.remove(searchParamSet, searchParamString);
        if (_.enabled === enabled) return Option.none();
        return Option.some<MessageInitShape<typeof HttpSearchParamUpdateSchema>>({
          enabled,
          httpSearchParamId: _.httpSearchParamId,
        });
      }),
      (_) => searchParamCollection.utils.update(_),
    );

    const lastOrder = pipe(
      await queryCollection((_) =>
        _.from({ item: searchParamCollection })
          .orderBy((_) => _.item.order, 'desc')
          .select((_) => ({ order: _.item.order }))
          .limit(1)
          .findOne(),
      ),
      Array.head,
      Option.map((_) => _.order),
      Option.getOrElse(() => 0),
    );

    const orderSpacing = (Protobuf.MAX_FLOAT - lastOrder) / (MutableHashSet.size(searchParamSet) + 1);

    pipe(
      Array.fromIterable(searchParamSet),
      Array.map((_, index): MessageInitShape<typeof HttpSearchParamInsertSchema> => {
        const separator = _.indexOf('=');
        return {
          enabled: true,
          httpId,
          httpSearchParamId: Ulid.generate().bytes,
          key: separator ? _.slice(0, separator) : _,
          order: lastOrder + orderSpacing * (index + 1),
          value: separator ? _.slice(separator + 1) : '',
        };
      }),
      (_) => searchParamCollection.utils.insert(_),
    );
  });

  return (
    <div className='shadow-xs flex flex-1 items-center gap-3 rounded-lg border border-slate-300 px-3 py-2'>
      <Select
        aria-label='Method'
        items={pipe(Struct.omit(HttpMethodSchema.value, 0), Record.values)}
        onSelectionChange={(method) => {
          if (typeof method !== 'number') return;
          httpCollection.utils.update({ httpId, method });
        }}
        selectedKey={method}
        triggerClassName={tw`border-none p-0`}
      >
        {(_) => (
          <SelectItem id={_.number} textValue={_.localName}>
            <MethodBadge method={_.number} size='lg' />
          </SelectItem>
        )}
      </Select>

      <Separator className={tw`h-7`} orientation='vertical' />

      <ReferenceFieldRHF
        aria-label='URL'
        className={tw`flex-1 border-none font-medium tracking-tight`}
        control={form.control}
        kind='StringExpression'
        name='urlString'
        onBlur={() => void submit()}
      />
    </div>
  );
};
