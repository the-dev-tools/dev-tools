import { Schema } from '@effect/schema';
import backgroundImage from 'data-base64:@/../assets/background.png';
import { Array, Clock, Duration, Effect, flow, HashMap, Match, Option, pipe, String, Struct, Tuple } from 'effect';
import * as React from 'react';
import * as RAC from 'react-aria-components';
import * as FeatherIcons from 'react-icons/fi';
import { twMerge } from 'tailwind-merge';

import * as Auth from '@/auth';
import * as Postman from '@/postman';
import * as Recorder from '@/recorder';
import { Runtime } from '@/runtime';
import * as Storage from '@/storage';
import * as UI from '@/ui';
import * as Utils from '@/utils';
import { tw } from '@/utils';

import '@fontsource-variable/lexend-deca';
import './style.css';

interface LayoutProps {
  children: React.ReactNode;
  className?: string;
}

const Layout = ({ className, children }: LayoutProps) => (
  <div className='relative z-0 h-[600px] w-[800px] overflow-hidden border border-slate-300 bg-slate-50 font-sans'>
    <div className='absolute inset-x-0 top-0 -z-10 bg-slate-50'>
      <img src={backgroundImage} alt='Background' className='mix-blend-luminosity' />
      <div className='absolute inset-0 shadow-[inset_0_0_2rem_2rem_var(--tw-shadow-color)] shadow-slate-50' />
    </div>

    <div className={twMerge('size-full', className)}>{children}</div>
  </div>
);

class LoginFormData extends Schema.Class<LoginFormData>('LoginFormData')({
  email: Schema.String,
}) {}

const LoginPage = () => {
  const [loading, setLoading] = React.useState(false);
  return (
    <Layout>
      <RAC.Form
        className='flex size-full flex-col items-center justify-center px-44'
        onSubmit={(event) =>
          Effect.gen(function* () {
            event.preventDefault();
            const { email } = yield* pipe(
              new FormData(event.currentTarget),
              Object.fromEntries,
              Schema.decode(LoginFormData),
            );
            setLoading(true);
            yield* Auth.loginInit(email);
            setLoading(false);
          }).pipe(Runtime.runPromise)
        }
      >
        <UI.Illustrations.Logo className='mb-2 h-16 w-auto' />
        <h1 className='mb-1 text-center text-4xl font-semibold uppercase leading-tight'>Dev Tools</h1>
        <h2 className='mb-10 w-64 text-center text-sm leading-snug'>
          Create your account and get your APIs call in seconds
        </h2>
        <RAC.TextField className='mb-6 w-full' name='email' type='email' isRequired>
          <RAC.Label className='mb-2 block'>Email</RAC.Label>
          <RAC.Input
            className={(renderProps) =>
              UI.FocusRing.styles({
                ...renderProps,
                className: [
                  tw`w-full rounded-lg border bg-white px-3 py-2 text-sm leading-tight text-slate-500`,
                  !renderProps.isFocused && tw`border-slate-300`,
                ],
              })
            }
          />
          <RAC.FieldError className='mt-2 block text-sm leading-none text-red-700' />
        </RAC.TextField>
        <UI.Button.Main className='w-full' type='submit'>
          {loading && <FeatherIcons.FiLoader className='animate-spin' />}
          Get Started
        </UI.Button.Main>
      </RAC.Form>
    </Layout>
  );
};

interface RecorderLayoutProps {
  children: React.ReactNode;
  headerSlot?: React.ReactNode;
}

const RecorderLayout = ({ children, headerSlot }: RecorderLayoutProps) => (
  <Layout className='flex flex-col divide-y divide-slate-300'>
    <div className='flex items-center gap-2 p-4'>
      <UI.Illustrations.Logo className='h-6 w-auto' />
      <h1 className='text-xl font-medium uppercase leading-tight'>Dev Tools</h1>
      <div className='h-9 flex-1' />
      {headerSlot}
    </div>
    {children}
  </Layout>
);

const IntroPage = () => (
  <RecorderLayout>
    <div className='flex min-h-0 flex-1 flex-col items-center justify-center gap-6'>
      <UI.Illustrations.IntroIcon />
      <div className='text-center'>
        <h2 className='mb-2 text-2xl font-medium leading-tight'>Get your API quicker</h2>
        <h3 className='text-sm leading-5'>Click the record to start the record</h3>
      </div>
      <UI.Button.Main onPress={() => void Recorder.start.pipe(Effect.ignoreLogged, Runtime.runPromise)}>
        Start Recording
      </UI.Button.Main>
    </div>
  </RecorderLayout>
);

const SelectionSchema = Schema.Union(Schema.Literal('all'), Schema.Set(Schema.Union(Schema.String, Schema.Number)));

const RecorderPage = () => {
  const collection = Recorder.useCollection();
  const tabId = Recorder.useTabId();

  const [searchTerm, setSearchTerm] = React.useState('');

  const filteredNavigations = (() => {
    if (searchTerm === '') return collection.item;

    const filterHosts = (navigation: Postman.Item) => (hosts: Postman.Item['item']) =>
      Array.filter(hosts ?? [], (host) => {
        if (!navigation.name || !host.name) return false;
        const searchString = String.toLowerCase(searchTerm);
        return pipe(navigation.name + host.name, String.toLowerCase, String.includes(searchString));
      });

    return pipe(
      collection.item,
      Array.map((_) => Struct.evolve(_, { item: filterHosts(_) })),
      Array.filter((navigation) => (navigation.item?.length ?? 0) > 0),
    );
  })();

  const indexMap = React.useMemo(() => {
    const itemTuples =
      <Key extends PropertyKey, PreviousIndex>(key: Key, previousIndex: PreviousIndex) =>
      (items: Postman.Item['item']) =>
        Array.map(items ?? [], (item, index) =>
          Tuple.make(item.id, {
            item,
            index: { ...previousIndex, ...Utils.keyValue(key, index) },
          }),
        );

    const mapItemTuples =
      <Key extends PropertyKey>(key: Key) =>
      <PreviousKey extends PropertyKey, PreviousIndex>(
        input: ReturnType<ReturnType<typeof itemTuples<PreviousKey, PreviousIndex>>>,
      ) =>
        pipe(input, Array.flatMap(flow(Tuple.getSecond, ({ item, index }) => itemTuples(key, index)(item.item))));

    const hostTuples = pipe(collection.item, itemTuples('navigation', {}), mapItemTuples('host'));

    return {
      hosts: pipe(hostTuples, HashMap.fromIterable),
      requests: pipe(hostTuples, mapItemTuples('request'), HashMap.fromIterable),
    };
  }, [collection.item]);

  const [hostsSelectionMaybe, setHostsSelection] = Storage.useState(Storage.Local, 'HostsSelection', SelectionSchema);
  const hostsSelection = Option.getOrElse(hostsSelectionMaybe, () => new Set<string | number>());

  const selectedHost = pipe(
    hostsSelection,
    Option.liftPredicate((_) => _ !== 'all'),
    Option.map((_) => _.values().next().value as Postman.Item['id']),
    Option.flatMap((_) => HashMap.get(indexMap.hosts, _)),
    Option.map(Struct.get('item')),
  );

  const filteredRequests = pipe(
    selectedHost,
    Option.flatMap(flow(Struct.get('item'), Option.fromNullable)),
    Option.map((requests) => {
      if (searchTerm === '') return requests;
      return Array.filter(requests, (request: Postman.Item) => {
        if (!request.name) return false;
        const searchString = String.toLowerCase(searchTerm);
        return pipe(request.name, String.toLowerCase, String.includes(searchString));
      });
    }),
    Option.getOrElse(() => []),
  );

  const [requestsSelectionMaybe, setRequestsSelection] = Storage.useState(
    Storage.Local,
    'RequestsSelection',
    SelectionSchema,
  );
  const requestsSelection = Option.getOrElse(requestsSelectionMaybe, () => new Set<string | number>());

  const selectedCollection = (): Postman.Collection => {
    if (requestsSelection === 'all') return collection;

    interface SelectedItem extends Omit<Postman.Item, 'item'> {
      readonly item?: Option.Option<SelectedItem>[];
    }

    const emptySelectedItems = <T extends Pick<Postman.Item, 'item'>>(item: T) =>
      Struct.evolve(item, {
        item: (): Option.Option<SelectedItem>[] => Array.makeBy(item.item?.length ?? 0, () => Option.none()),
      });

    const selectCollection = (requests: HashMap.HashMap.Value<(typeof indexMap)['requests']>[]) =>
      Array.reduce(requests, emptySelectedItems(collection), (selectedCollection, request) =>
        Option.gen(function* () {
          const navigation = yield* Array.get(collection.item, request.index.navigation);
          const host = yield* Array.get(navigation.item ?? [], request.index.host);

          let selectedNavigation: SelectedItem = pipe(
            selectedCollection.item,
            Array.get(request.index.navigation),
            Option.flatten,
            Option.getOrElse(() => emptySelectedItems(navigation)),
          );

          const selectedHost: SelectedItem = pipe(
            selectedNavigation.item ?? [],
            Array.get(request.index.host),
            Option.flatten,
            Option.getOrElse(() => emptySelectedItems(host)),
            Struct.evolve({
              item: (_) =>
                Array.replace(_ ?? [], request.index.request, pipe(request.item, Struct.omit('item'), Option.some)),
            }),
          );

          selectedNavigation = Struct.evolve(selectedNavigation, {
            item: (_) => Array.replace(_ ?? [], request.index.host, Option.some(selectedHost)),
          });

          return Struct.evolve(selectedCollection, {
            item: (_) => Array.replace(_, request.index.navigation, Option.some(selectedNavigation)),
          });
        }).pipe(Option.getOrElse(() => selectedCollection)),
      );

    const evolveItems =
      <K,>(map: (item: SelectedItem) => K) =>
      <A extends SelectedItem>(item: A) =>
        Struct.evolve(item, {
          item: (_) => pipe(_ ?? [], Array.getSomes, (_) => Array.map(_ as SelectedItem[], map)),
        });

    return pipe(
      requestsSelection.values(),
      Array.fromIterable,
      Array.map((_) => pipe(_ as Postman.Item['id'], (_) => HashMap.get(indexMap.requests, _))),
      Array.getSomes,
      selectCollection,
      evolveItems(evolveItems(evolveItems(Struct.omit('item')))),
    );
  };

  const exportCollection = Effect.gen(function* () {
    const file = yield* pipe(
      selectedCollection(),
      Schema.encode(Postman.Collection),
      Effect.map(JSON.stringify),
      Effect.map((_) => new Blob([_], { type: 'text/json' })),
    );
    const link = document.createElement('a');
    link.href = URL.createObjectURL(file);
    link.download = `postman-collection.json`;
    link.click();
    URL.revokeObjectURL(link.href);
  });

  const currentTimeMillis = pipe(Clock.currentTimeMillis, Runtime.runSync);

  if (Option.isNone(tabId) && collection.item.length === 0) return <IntroPage />;

  return (
    <RecorderLayout
      headerSlot={
        <RAC.SearchField value={searchTerm} onChange={setSearchTerm} className='group w-80' aria-label='Search'>
          <RAC.Group
            className={(renderProps) =>
              UI.FocusRing.styles({
                ...renderProps,
                className: [
                  tw`flex items-center rounded-lg border bg-white px-3 text-slate-500`,
                  !renderProps.isFocusWithin && tw`border-slate-300`,
                ],
              })
            }
          >
            <FeatherIcons.FiSearch className='size-4' />
            <RAC.Input
              className='min-w-0 flex-1 p-2 text-sm leading-tight outline outline-0 [&::-webkit-search-cancel-button]:hidden'
              placeholder='Search'
            />
            <RAC.Button className='rounded-full bg-gray-100 p-1 opacity-100 transition-opacity group-rac-empty:invisible group-rac-empty:opacity-0'>
              <FeatherIcons.FiX className='size-4' />
            </RAC.Button>
          </RAC.Group>
        </RAC.SearchField>
      }
    >
      <div className='flex min-h-0 flex-1 divide-x divide-slate-300 '>
        <div className='flex flex-1 flex-col items-start gap-4 overflow-auto p-4'>
          <h2 className='text-2xl font-medium leading-7'>Visited pages</h2>

          <RAC.ListBox
            items={filteredNavigations}
            selectionMode='single'
            onSelectionChange={flow(setHostsSelection, Runtime.runPromise)}
            selectedKeys={hostsSelection}
            aria-label='Visited pages'
            className='flex w-full flex-col gap-4'
          >
            {(navigation) => (
              <RAC.Section id={navigation.id ?? ''}>
                <RAC.Header className='truncate rounded-t-lg border border-slate-200 bg-white px-4 py-3 text-xs font-medium'>
                  {navigation.name}
                </RAC.Header>
                <RAC.Collection items={navigation.item ?? []}>
                  {(host) => (
                    <RAC.ListBoxItem
                      id={host.id ?? ''}
                      textValue={host.name ?? ''}
                      className={(renderProps) =>
                        UI.FocusRing.styles({
                          ...renderProps,
                          className: [
                            tw`group relative -mt-px flex cursor-pointer items-center gap-2.5 overflow-auto border bg-slate-50 px-4 py-6 text-sm transition-[border-color,outline-color,outline-width,background-color] last:rounded-b-lg odd:bg-white rac-selected:bg-indigo-100`,
                            !renderProps.isFocused && tw`border-slate-200`,
                          ],
                        })
                      }
                    >
                      <div className='absolute inset-y-0 left-0 w-0 bg-indigo-700 transition-[width] group-rac-selected:w-0.5' />
                      <RAC.Text
                        slot='label'
                        className='flex-1 truncate text-slate-500 transition-colors group-rac-selected:text-indigo-700'
                      >
                        {host.name}
                      </RAC.Text>
                      <div className='rounded-full border border-slate-200 bg-slate-50 px-2.5 py-0.5 text-slate-700 transition-colors group-rac-selected:border-indigo-200 group-rac-selected:bg-indigo-50 group-rac-selected:text-indigo-700'>
                        {host.item?.length ?? 0} calls
                      </div>
                      <FeatherIcons.FiChevronRight className='size-5 text-slate-500 transition-colors group-rac-selected:text-indigo-700' />
                    </RAC.ListBoxItem>
                  )}
                </RAC.Collection>
              </RAC.Section>
            )}
          </RAC.ListBox>
        </div>

        <div className='flex flex-1 flex-col items-start gap-4 overflow-auto p-4'>
          <h2 className='text-2xl font-medium leading-7'>API Calls</h2>

          <RAC.ListBox
            items={filteredRequests}
            selectionMode='multiple'
            selectedKeys={requestsSelection}
            onSelectionChange={flow(setRequestsSelection, Runtime.runPromise)}
            aria-label='API Calls'
            className={(renderProps) =>
              UI.FocusRing.styles({
                ...renderProps,
                className: [tw`w-full`, renderProps.isEmpty && tw`min-h-0 flex-1`],
              })
            }
            renderEmptyState={() => (
              <div className='flex h-full flex-col items-center justify-center'>
                <UI.Illustrations.Collection className='mb-6' />
                <h3 className='mb-2 text-xl font-semibold leading-tight'>No calls yet</h3>
                <span className='text-sm leading-5'>{"Let's try another one"}</span>
              </div>
            )}
          >
            {(request) => (
              <RAC.ListBoxItem
                id={request.id ?? ''}
                textValue={request.name ?? ''}
                className={(renderProps) =>
                  UI.FocusRing.styles({
                    ...renderProps,
                    className: [
                      tw`-mt-px grid cursor-pointer grid-cols-[auto_auto_1fr_auto] grid-rows-[auto_auto] items-center gap-y-1.5 border bg-slate-50 p-4 text-slate-500 transition-[border-color,outline-color,outline-width,background-color] first:mt-0 first:rounded-t-lg first:border-t last:rounded-b-lg even:bg-white rac-selected:bg-indigo-100`,
                      !renderProps.isFocused && tw`border-slate-200`,
                    ],
                  })
                }
              >
                {({ isSelected }) => (
                  <>
                    <RAC.Checkbox
                      isReadOnly
                      excludeFromTabOrder
                      isSelected={isSelected}
                      aria-label={request.name ?? ''}
                      className='group relative row-span-2'
                    >
                      <div className='mr-3 flex size-5 cursor-pointer items-center justify-center rounded border border-slate-300 text-white transition-colors group-rac-selected:border-transparent group-rac-selected:bg-indigo-600'>
                        {isSelected && <FeatherIcons.FiCheck />}
                      </div>
                    </RAC.Checkbox>

                    {pipe(
                      request.request,
                      Option.liftPredicate(Schema.is(Postman.RequestClass)),
                      Option.map(({ method }) =>
                        pipe(
                          method,
                          Match.value,
                          Match.when('GET', () => tw`border-orange-200 bg-orange-50 text-orange-900`),
                          Match.when('POST', () => tw`border-green-200 bg-green-50 text-green-900`),
                          Match.orElse(() => tw`border-slate-200 bg-slate-50 text-slate-700`),
                          (_) => [method ?? 'ETC', _] as const,
                        ),
                      ),
                      Option.map(([method, className]) => (
                        <div
                          key={null}
                          className={twMerge(
                            'col-start-2 row-start-2 mr-1.5 rounded border px-2 py-1 text-xs leading-tight',
                            className,
                          )}
                        >
                          {pipe(method, String.toLowerCase, String.capitalize)}
                        </div>
                      )),
                      Option.getOrElse(() => null),
                    )}

                    {pipe(
                      request.name ?? '',
                      Utils.URL.make,
                      Effect.map((url) => (
                        <>
                          <span className='col-span-2 col-start-2 truncate text-xs leading-none text-indigo-600'>
                            {url.host}
                          </span>
                          <span className='col-start-3 row-start-2 truncate text-sm' title={url.href}>
                            {url.pathname}
                          </span>
                        </>
                      )),
                      Runtime.runSync,
                    )}

                    {Effect.gen(function* () {
                      const variable = yield* Array.findFirst(request.variable ?? [], (_) => _.key === 'timestamp');
                      const timestamp = yield* pipe(variable, Struct.get('value'), Schema.decodeUnknown(Schema.Number));
                      const duration = Duration.subtract(currentTimeMillis, Duration.seconds(timestamp));

                      const sec = Math.floor(Duration.toSeconds(duration));
                      if (sec < 60) return `${sec.toString()} sec`;

                      const min = Math.floor(sec / 60);
                      if (min < 60) return `${min.toString()} min`;

                      const hr = Math.floor(min / 60);
                      if (hr < 24) return `${hr.toString()} hr`;

                      const days = Math.floor(hr / 24);
                      return `${days.toString()} days`;
                    }).pipe(
                      Effect.match({
                        onFailure: () => null,
                        onSuccess: (_) => (
                          <span className='col-start-4 row-span-2 text-xs font-light leading-5'>{_} ago</span>
                        ),
                      }),
                      Runtime.runSync,
                    )}
                  </>
                )}
              </RAC.ListBoxItem>
            )}
          </RAC.ListBox>
        </div>
      </div>

      <div className='flex items-center gap-3 bg-white p-4'>
        {Option.match(tabId, {
          onNone: () => (
            <>
              <div className='size-4 rounded-full border-2 border-slate-200 bg-slate-600' />
              <h1 className='text-base font-medium leading-tight'>Recording paused</h1>
            </>
          ),
          onSome: () => (
            <>
              <div className='size-4 rounded-full border-2 border-red-200 bg-red-500' />
              <h1 className='text-base font-medium leading-tight'>Recording API Calls</h1>
            </>
          ),
        })}

        <div className='flex-1' />

        <UI.Button.Main
          onPress={() => void Auth.logout.pipe(Effect.ignoreLogged, Runtime.runPromise)}
          variant='secondary gray'
        >
          Log out
        </UI.Button.Main>

        <UI.Button.Main
          onPress={() => void Recorder.setReset(true).pipe(Effect.ignoreLogged, Runtime.runPromise)}
          variant='secondary gray'
        >
          Reset
        </UI.Button.Main>

        {Option.match(tabId, {
          onNone: () => (
            <UI.Button.Main
              onPress={() => void Recorder.start.pipe(Effect.ignoreLogged, Runtime.runPromise)}
              variant='secondary color'
            >
              Resume
              <FeatherIcons.FiPlayCircle />
            </UI.Button.Main>
          ),
          onSome: () => (
            <UI.Button.Main
              onPress={() => void Recorder.stop.pipe(Effect.ignoreLogged, Runtime.runPromise)}
              variant='secondary color'
            >
              Pause
              <FeatherIcons.FiPauseCircle />
            </UI.Button.Main>
          ),
        })}

        <UI.Button.Main
          onPress={() => void exportCollection.pipe(Effect.ignoreLogged, Runtime.runPromise)}
          variant='primary'
        >
          Export
        </UI.Button.Main>
      </div>
    </RecorderLayout>
  );
};

const PopupPage = () => {
  const [loggedInMaybe] = Auth.useLoggedIn();
  const loggedIn = Option.getOrElse(loggedInMaybe, () => false);
  return loggedIn ? <RecorderPage /> : <LoginPage />;
};

export default PopupPage;
