import { Schema } from '@effect/schema';
import backgroundImage from 'data-base64:@/../assets/background.jpg';
import { Array, Effect, flow, Match, Number, Option, pipe, Record, String, Struct } from 'effect';
import * as React from 'react';
import * as RAC from 'react-aria-components';
import { twMerge } from 'tailwind-merge';

import * as Postman from '@/postman';
import * as Recorder from '@/recorder';
import { Runtime } from '@/runtime';
import * as UI from '@/ui';

import '@fontsource-variable/lexend-deca';
import './style.css';

class HostSelectionKey extends Schema.Class<HostSelectionKey>('HostIndex')({
  id: Schema.String,
  navigationIndex: Schema.Number,
  hostIndex: Schema.Number,
}) {}

class RequestSelectionKey extends HostSelectionKey.extend<RequestSelectionKey>('RequestSelectionKey')({
  requestIndex: Schema.Number,
}) {}

const PopupPageNew = () => {
  const collection = Recorder.useCollection();
  const tabId = Recorder.useTabId();

  const [hostsSelection, setHostsSelection] = React.useState<RAC.Selection>(new Set());

  let selectedHost = Option.none<[Postman.Item, HostSelectionKey]>();
  if (hostsSelection !== 'all' && hostsSelection.size > 0) {
    const selection = pipe(
      hostsSelection.values().next().value as string,
      JSON.parse,
      Schema.decodeUnknownSync(HostSelectionKey),
    );
    selectedHost = pipe(
      collection.item,
      Array.get(selection.navigationIndex),
      Option.flatMap(flow(Struct.get('item'), Option.fromNullable)),
      Option.flatMap(Array.get(selection.hostIndex)),
      Option.map((_) => [_, selection]),
    );
  }

  const [requestsSelection, setRequestsSelection] = React.useState<RAC.Selection>(new Set());

  const selectedCollection = Effect.gen(function* () {
    if (requestsSelection === 'all') return collection;

    const selections = yield* pipe(
      requestsSelection.values(),
      Array.fromIterable,
      Array.map((_) => pipe(_ as string, JSON.parse, Schema.decodeUnknown(RequestSelectionKey))),
      Effect.all,
    );

    const navigationsIndexGroup = pipe(
      Array.groupBy(selections, (_) => _.navigationIndex.toString()),
      Record.map(
        flow(
          Array.groupBy((_) => _.hostIndex.toString()),
          Record.map(Array.map(Struct.get('requestIndex'))),
        ),
      ),
    );

    const filterItemsByIndexGroup =
      <Next,>(group: Record<string, Next>, filter: (next: Next) => (item: Postman.Item['item']) => Postman.Item[]) =>
      (item: Postman.Item['item']) =>
        pipe(
          Record.toEntries(group),
          Array.map(([index, next]) =>
            pipe(
              Number.parse(index),
              Option.flatMap((_) => Array.get(item ?? [], _)),
              Option.map(Struct.evolve({ item: filter(next) })),
            ),
          ),
          Array.getSomes,
        );

    const filterRequestsByIndex = (indexes: number[]) => (hosts: Postman.Item['item']) =>
      pipe(
        Array.map(indexes, (_) => Array.get(hosts ?? [], _)),
        Array.getSomes,
      );

    const selectedCollection = Struct.evolve(collection, {
      item: filterItemsByIndexGroup(navigationsIndexGroup, (hostsGroup) =>
        filterItemsByIndexGroup(hostsGroup, filterRequestsByIndex),
      ),
    });

    return selectedCollection;
  });

  const exportCollection = Effect.gen(function* () {
    const file = yield* pipe(
      selectedCollection,
      Effect.flatMap(Schema.encode(Postman.Collection)),
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
    <div className='relative flex h-[600px] w-[800px] flex-col divide-y divide-slate-300 border border-slate-300 font-sans'>
      <div className='absolute inset-0 -z-10 bg-slate-50' />
      <img src={backgroundImage} alt='Background' className='absolute inset-x-0 top-0 -z-10' />

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
      </div>

      <div className='flex min-h-0 flex-1 divide-x divide-slate-300 '>
        <div className='flex flex-1 flex-col items-start gap-4 overflow-auto p-4'>
          <h2 className='text-2xl font-medium leading-7'>Visited pages</h2>

          <RAC.ListBox
            items={collection.item.map((item, index) => [item, index] as const)}
            selectionMode='single'
            onSelectionChange={setHostsSelection}
            selectedKeys={hostsSelection}
            aria-label='Visited pages'
            className='flex w-full flex-col gap-4'
          >
            {([navigation, navigationIndex]) => (
              <RAC.Section id={navigation.id ?? '' + navigationIndex.toString()}>
                <RAC.Header className='truncate rounded-t-lg border border-slate-200 bg-white px-4 py-3 text-xs font-medium'>
                  {navigation.name}
                </RAC.Header>
                <RAC.Collection items={(navigation.item ?? []).map((item, index) => [item, index] as const)}>
                  {([host, hostIndex]) => (
                    <RAC.ListBoxItem
                      id={pipe(
                        HostSelectionKey.make({ id: host.id ?? '', navigationIndex, hostIndex }),
                        Schema.encodeSync(HostSelectionKey),
                        JSON.stringify,
                      )}
                      textValue={host.name ?? ''}
                      className='group relative flex cursor-pointer items-center border-x border-b border-slate-200 bg-slate-50 px-4 py-6 text-sm transition-colors last:rounded-b-lg odd:bg-white rac-selected:bg-indigo-100'
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

          {Option.match(selectedHost, {
            onNone: () => <p>Select recorded page</p>,
            onSome: ([host, { navigationIndex, hostIndex }]) => (
              <RAC.ListBox
                items={(host.item ?? []).map((item, index) => [item, index] as const)}
                selectionMode='multiple'
                selectedKeys={requestsSelection}
                onSelectionChange={setRequestsSelection}
                aria-label='API Calls'
                className='w-full'
              >
                {([request, requestIndex]) => (
                  <RAC.ListBoxItem
                    id={pipe(
                      RequestSelectionKey.make({ id: request.id ?? '', hostIndex, navigationIndex, requestIndex }),
                      Schema.encodeSync(RequestSelectionKey),
                      JSON.stringify,
                    )}
                    textValue={request.name ?? ''}
                    className='flex cursor-pointer items-center border-x border-b border-slate-200 bg-slate-50 px-4 py-6 text-slate-500 transition-colors first:rounded-t-lg first:border-t last:rounded-b-lg even:bg-white rac-selected:bg-indigo-100'
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
                              Match.when('GET', () => 'border-orange-200 bg-orange-50 text-orange-900'),
                              Match.when('POST', () => 'border-green-200 bg-green-50 text-green-900'),
                              Match.orElse(() => 'border-slate-200 bg-slate-50 text-slate-700'),
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
            ),
          })}
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
          onPress={() => void Recorder.reset.pipe(Effect.ignoreLogged, Runtime.runPromise)}
          variant='secondary'
        >
          Reset
        </UI.Button.Main>

        <UI.Button.Main
          onPress={() => void exportCollection.pipe(Effect.ignoreLogged, Runtime.runPromise)}
          variant='secondary'
        >
          Export
        </UI.Button.Main>
      </div>
    </div>
  );
};

export default PopupPageNew;
