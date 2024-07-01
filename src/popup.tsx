import { Schema } from '@effect/schema';
import backgroundImage from 'data-base64:@/../assets/background.jpg';
import { Array, Effect, flow, Match, Option, pipe, Struct } from 'effect';
import * as RAC from 'react-aria-components';

import * as Postman from '@/postman';
import * as Recorder from '@/recorder';
import { Runtime } from '@/runtime';
import * as UI from '@/ui';

import './style.css';
import '@fontsource-variable/lexend-deca';

const PopupPageNew = () => {
  const navigations = Recorder.useNavigations();
  const tabId = Recorder.useTabId();

  const lastNavigationItems = pipe(
    navigations,
    Array.last,
    Option.map(Struct.get('item')),
    Option.flatMap(Option.fromNullable),
    Option.flatMap(Array.last),
    Option.map(Struct.get('item')),
    Option.flatMap(Option.fromNullable),
    Option.getOrElse(() => []),
  );

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
            items={navigations}
            selectionMode='single'
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
                      className='group relative flex items-center border-x border-b border-slate-200 bg-slate-50 px-4 py-6 text-sm last:rounded-b-lg odd:bg-white rac-selected:bg-indigo-100'
                    >
                      <div className='absolute inset-y-0 left-0 w-0 bg-indigo-700 transition-[width] group-aria-selected:w-0.5' />
                      <RAC.Text
                        slot='label'
                        className='flex-1 truncate text-slate-500 group-aria-selected:text-indigo-700'
                      >
                        {host.name}
                      </RAC.Text>
                      <div className='rounded-full border border-slate-200 bg-slate-50 px-2.5 py-0.5 text-slate-700 group-aria-selected:border-indigo-200 group-aria-selected:bg-indigo-50 group-aria-selected:text-indigo-700'>
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

          <div className='w-full'>
            {lastNavigationItems.map((_, index) => (
              <div
                key={(_.id ?? '') + index.toString()}
                className='flex items-center border-x border-b border-slate-200 bg-slate-50 px-4 py-6 text-slate-500 first:rounded-t-lg first:border-t last:rounded-b-lg even:bg-white'
              >
                {pipe(
                  _.request,
                  Option.liftPredicate(Schema.is(Postman.RequestClass)),
                  Option.flatMap(
                    flow(
                      Match.value,
                      Match.when(
                        { method: 'GET' },
                        () => ['Get', 'border-orange-200 bg-orange-50 text-orange-900'] as const,
                      ),
                      Match.when(
                        { method: 'POST' },
                        () => ['Post', 'border-green-200 bg-green-50 text-green-900'] as const,
                      ),
                      Match.option,
                    ),
                  ),
                  Option.map(([title, className]) => (
                    <div key={null} className={`mr-1.5 rounded border px-2 py-1 text-xs leading-tight ${className}`}>
                      {title}
                    </div>
                  )),
                  Option.getOrElse(() => null),
                )}

                <span className='flex-1 truncate text-sm'>{_.name}</span>
              </div>
            ))}
          </div>
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
