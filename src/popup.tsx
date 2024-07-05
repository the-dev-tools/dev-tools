import { Schema } from '@effect/schema';
import backgroundImage from 'data-base64:@/../assets/background.png';
import { Array, Effect, flow, HashMap, Match, Option, pipe, String, Struct, Tuple } from 'effect';
import * as React from 'react';
import * as RAC from 'react-aria-components';
import * as FeatherIcons from 'react-icons/fi';
import { twMerge } from 'tailwind-merge';

import * as Auth from '@/auth';
import * as Postman from '@/postman';
import * as Recorder from '@/recorder';
import { Runtime } from '@/runtime';
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
  <div className='relative z-0 h-[600px] w-[800px] border border-slate-300 bg-slate-50 font-sans'>
    <div className='absolute inset-x-0 top-0 -z-10 bg-slate-50'>
      <img src={backgroundImage} alt='Background' className='mix-blend-luminosity' />
      <div className='absolute inset-0 shadow-[inset_0_0_2rem_2rem_var(--tw-shadow-color)] shadow-slate-50' />
    </div>

    <div className={twMerge('size-full', className)}>{children}</div>
  </div>
);

const LoginPage = () => {
  const [email, setEmail] = React.useState('');
  return (
    <Layout className='flex flex-col justify-center px-44'>
      <RAC.TextField value={email} onChange={setEmail} type='email'>
        <RAC.Label className='mb-2 block'>Email</RAC.Label>
        <RAC.Input
          className={(renderProps) =>
            UI.FocusRing.styles({
              ...renderProps,
              className: tw`mb-6 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm leading-tight text-slate-500`,
            })
          }
        />
      </RAC.TextField>
      <UI.Button.Main onPress={() => void Auth.loginInit(email).pipe(Effect.ignoreLogged, Runtime.runPromise)}>
        Get Started
      </UI.Button.Main>
    </Layout>
  );
};

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

  const [hostsSelection, setHostsSelection] = React.useState<RAC.Selection>(new Set());

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

  const [requestsSelection, setRequestsSelection] = React.useState<RAC.Selection>(new Set());

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

  return (
    <Layout className='flex flex-col divide-y divide-slate-300'>
      <div className='flex items-center gap-2 px-4 py-5'>
        {Option.match(tabId, {
          onNone: () => <h1 className='text-xl font-medium leading-6'>API Recorder</h1>,
          onSome: () => (
            <>
              <div className='size-3 rounded-full border-2 border-red-200 bg-red-500' />
              <h1 className='text-xl font-medium leading-6'>Recording API Calls</h1>
            </>
          ),
        })}

        <div className='flex-1' />

        <RAC.SearchField value={searchTerm} onChange={setSearchTerm} className='group w-80' aria-label='Search'>
          <RAC.Group
            className={(renderProps) =>
              UI.FocusRing.styles({
                ...renderProps,
                className: tw`flex items-center rounded-lg border border-slate-300 bg-white px-3 text-slate-500`,
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
      </div>

      <div className='flex min-h-0 flex-1 divide-x divide-slate-300 '>
        <div className='flex flex-1 flex-col items-start gap-4 overflow-auto p-4'>
          <h2 className='text-2xl font-medium leading-7'>Visited pages</h2>

          <RAC.ListBox
            items={filteredNavigations}
            selectionMode='single'
            onSelectionChange={setHostsSelection}
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
                          className: tw`group relative flex cursor-pointer items-center border-x border-b border-slate-200 bg-slate-50 px-4 py-6 text-sm transition-colors last:rounded-b-lg odd:bg-white rac-selected:bg-indigo-100`,
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
            onSelectionChange={setRequestsSelection}
            aria-label='API Calls'
            className='w-full'
          >
            {(request) => (
              <RAC.ListBoxItem
                id={request.id ?? ''}
                textValue={request.name ?? ''}
                className={(renderProps) =>
                  UI.FocusRing.styles({
                    ...renderProps,
                    className: tw`flex cursor-pointer items-center border-x border-b border-slate-200 bg-slate-50 px-4 py-6 text-slate-500 transition-colors first:rounded-t-lg first:border-t last:rounded-b-lg even:bg-white rac-selected:bg-indigo-100`,
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
                      className='group relative'
                    >
                      <div className='mr-3 flex size-5 cursor-pointer items-center justify-center rounded border border-slate-300 text-white transition-colors group-rac-selected:border-transparent group-rac-selected:bg-indigo-600'>
                        {isSelected ? 'V' : null}
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
                          className={twMerge('mr-1.5 rounded border px-2 py-1 text-xs leading-tight', className)}
                        >
                          {pipe(method, String.toLowerCase, String.capitalize)}
                        </div>
                      )),
                      Option.getOrElse(() => null),
                    )}

                    <span className='flex-1 truncate text-sm'>{request.name}</span>
                  </>
                )}
              </RAC.ListBoxItem>
            )}
          </RAC.ListBox>
        </div>
      </div>

      <div className='flex gap-3 bg-white p-4'>
        {Option.match(tabId, {
          onNone: () => (
            <UI.Button.Main onPress={() => void Recorder.start.pipe(Effect.ignoreLogged, Runtime.runPromise)}>
              Start
            </UI.Button.Main>
          ),
          onSome: () => (
            <UI.Button.Main onPress={() => void Recorder.stop.pipe(Effect.ignoreLogged, Runtime.runPromise)}>
              Stop
            </UI.Button.Main>
          ),
        })}

        <UI.Button.Main
          onPress={() => void Recorder.setReset(true).pipe(Effect.ignoreLogged, Runtime.runPromise)}
          variant='secondary'
        >
          Reset
        </UI.Button.Main>

        <UI.Button.Main
          onPress={() => void Auth.logout.pipe(Effect.ignoreLogged, Runtime.runPromise)}
          variant='secondary'
        >
          Log out
        </UI.Button.Main>

        <UI.Button.Main
          onPress={() => void exportCollection.pipe(Effect.ignoreLogged, Runtime.runPromise)}
          variant='secondary'
        >
          Export
        </UI.Button.Main>
      </div>
    </Layout>
  );
};

const PopupPage = () => {
  const [loggedInMaybe] = Auth.useLoggedIn();
  const loggedIn = Option.getOrElse(loggedInMaybe, () => false);
  return loggedIn ? <RecorderPage /> : <LoginPage />;
};

export default PopupPage;
