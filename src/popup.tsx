import { Schema } from '@effect/schema';
import backgroundImage from 'data-base64:@/../assets/background.jpg';
import { Array, Effect, flow, Match, Option, pipe, String, Struct } from 'effect';
import * as React from 'react';
import * as RAC from 'react-aria-components';
import { twMerge } from 'tailwind-merge';

import * as Postman from '@/postman';
import * as Recorder from '@/recorder';
import { Runtime } from '@/runtime';
import * as UI from '@/ui';

import '@fontsource-variable/lexend-deca';
import './style.css';

class HostIndex extends Schema.Class<HostIndex>('HostIndex')({
  id: Schema.String,
  index: Schema.Number,
  navigationIndex: Schema.Number,
}) {}

const PopupPageNew = () => {
  const navigations = Recorder.useNavigations();
  const tabId = Recorder.useTabId();

  const [selectedHosts, setSelectedHosts] = React.useState<RAC.Selection>(new Set());

  let selectedHostIndex = Option.none<Postman.Item>();
  if (typeof selectedHosts !== 'string' && selectedHosts.size > 0) {
    const { navigationIndex, index } = pipe(
      selectedHosts.values().next().value as string,
      JSON.parse,
      Schema.decodeUnknownSync(HostIndex),
    );
    selectedHostIndex = pipe(
      navigations,
      Array.get(navigationIndex),
      Option.flatMap(flow(Struct.get('item'), Option.fromNullable)),
      Option.flatMap(Array.get(index)),
    );
  }

  const [selectedRequests, setSelectedRequests] = React.useState<RAC.Selection>(new Set());

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
            items={navigations.map((item, index) => [item, index] as const)}
            selectionMode='single'
            onSelectionChange={setSelectedHosts}
            selectedKeys={selectedHosts}
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
                        HostIndex.make({ id: host.id ?? '', index: hostIndex, navigationIndex }),
                        Schema.encodeSync(HostIndex),
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

          {Option.match(selectedHostIndex, {
            onNone: () => <p>Select recorded page</p>,
            onSome: (host) => (
              <RAC.ListBox
                items={host.item ?? []}
                selectionMode='multiple'
                selectedKeys={selectedRequests}
                onSelectionChange={setSelectedRequests}
                aria-label='API Calls'
                className='w-full'
              >
                {(request) => (
                  <RAC.ListBoxItem
                    id={request.id ?? ''}
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
      </div>
    </div>
  );
};

export default PopupPageNew;
