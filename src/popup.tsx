import * as React from 'react';

import { useStorage } from '@plasmohq/storage/hook';

import * as Storage from '@/storage';

import './style.css';

const PopupPage = () => {
  const [recordingTabId, setRecordingTabId] = useStorage<number | null>(
    { instance: Storage.Local, key: Storage.RECORDING_TAB_ID },
    (_) => _ ?? null,
  );

  const startRecording = async () => {
    const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
    const tabId = tabs[0]?.id;
    if (!tabId) return;
    await setRecordingTabId(tabId);
  };

  const [calls] = useStorage<Storage.NetworkCall[]>(
    { instance: Storage.Local, key: Storage.RECORDED_CALLS },
    (_) => _ ?? [],
  );

  const [activeOrigin, setActiveOrigin] = React.useState<null | string>(null);

  const [searchTerm, setSearchTerm] = React.useState('');
  const [reloadOnStart, setReloadOnStart] = React.useState(false);

  const originCallCounts = Object.entries(
    calls.reduce<Record<string, number>>((result, value) => {
      const { origin } = new URL(value.url);
      if (!result[origin]) result[origin] = 0;
      result[origin]++;
      return result;
    }, {}),
  );

  const filteredOriginCallCounts = searchTerm
    ? originCallCounts.filter(([url]) => url.includes(searchTerm))
    : originCallCounts;

  return (
    <div className='flex h-[35rem] w-[50rem] flex-col divide-y divide-gray-100 bg-white'>
      {/* Header */}
      <div className='flex items-center justify-between p-2'>
        <h1>
          <svg
            className='relative -top-0.5 inline-flex size-5 animate-pulse text-red-600'
            xmlns='http://www.w3.org/2000/svg'
            fill='currentColor'
            width='800px'
            height='800px'
            viewBox='0 0 256 256'
          >
            <circle cx='127' cy='129' r='81' fillRule='evenodd' />
          </svg>
          Recording API calls
        </h1>
        <svg
          xmlns='http://www.w3.org/2000/svg'
          className='size-5 cursor-pointer text-gray-600'
          fill='none'
          viewBox='0 0 24 24'
          stroke='currentColor'
          onClick={() => void chrome.runtime.openOptionsPage()}
        >
          <path
            d='M10.255 4.18806C9.84269 5.17755 8.68655 5.62456 7.71327 5.17535C6.10289 4.4321 4.4321 6.10289 5.17535 7.71327C5.62456 8.68655 5.17755 9.84269 4.18806 10.255C2.63693 10.9013 2.63693 13.0987 4.18806 13.745C5.17755 14.1573 5.62456 15.3135 5.17535 16.2867C4.4321 17.8971 6.10289 19.5679 7.71327 18.8246C8.68655 18.3754 9.84269 18.8224 10.255 19.8119C10.9013 21.3631 13.0987 21.3631 13.745 19.8119C14.1573 18.8224 15.3135 18.3754 16.2867 18.8246C17.8971 19.5679 19.5679 17.8971 18.8246 16.2867C18.3754 15.3135 18.8224 14.1573 19.8119 13.745C21.3631 13.0987 21.3631 10.9013 19.8119 10.255C18.8224 9.84269 18.3754 8.68655 18.8246 7.71327C19.5679 6.10289 17.8971 4.4321 16.2867 5.17535C15.3135 5.62456 14.1573 5.17755 13.745 4.18806C13.0987 2.63693 10.9013 2.63693 10.255 4.18806Z'
            stroke='#323232'
            strokeWidth='2'
            strokeLinecap='round'
            strokeLinejoin='round'
          />
          <path
            d='M15 12C15 13.6569 13.6569 15 12 15C10.3431 15 9 13.6569 9 12C9 10.3431 10.3431 9 12 9C13.6569 9 15 10.3431 15 12Z'
            stroke='#323232'
            strokeWidth='2'
          />
        </svg>
      </div>

      {/* Search */}
      <div className='relative'>
        <svg
          className='pointer-events-none absolute left-4 top-3.5 size-5 text-gray-400'
          viewBox='0 0 20 20'
          fill='currentColor'
          aria-hidden='true'
        >
          <path
            fillRule='evenodd'
            d='M9 3.5a5.5 5.5 0 100 11 5.5 5.5 0 000-11zM2 9a7 7 0 1112.452 4.391l3.328 3.329a.75.75 0 11-1.06 1.06l-3.329-3.328A7 7 0 012 9z'
            clipRule='evenodd'
          />
        </svg>
        <input
          type='text'
          className='h-12 w-full border-0 bg-transparent pl-11 pr-4 text-gray-800 placeholder:text-gray-400 focus:ring-0'
          placeholder='Search...'
          value={searchTerm}
          onChange={(event) => void setSearchTerm(event.target.value)}
        />
      </div>

      {!recordingTabId ? (
        // Start screen
        <div className='pt-10'>
          <div className='relative mx-auto'>
            <div className='relative mx-auto -mb-16 flex size-16 animate-ping items-center justify-center rounded-full bg-red-500'></div>
            <button
              className='relative mx-auto flex size-16 items-center justify-center rounded-full bg-red-500 font-bold text-white focus:outline-none'
              onClick={() => void startRecording()}
            >
              Start
              <div className='absolute inset-0 size-full rounded-full border-4 border-red-500 opacity-50'></div>
            </button>
          </div>

          <div className='mt-8 flex'>
            <div className='mx-auto flex max-w-56 items-center'>
              <button
                type='button'
                className={`relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-indigo-600 focus:ring-offset-2 ${
                  reloadOnStart ? 'bg-indigo-600' : 'bg-gray-200'
                }`}
                role='switch'
                aria-checked={reloadOnStart}
                onClick={() => void setReloadOnStart(!reloadOnStart)}
              >
                <span
                  aria-hidden='true'
                  className={`pointer-events-none inline-block size-5 rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out ${reloadOnStart ? 'translate-x-5' : 'translate-x-0'}`}
                ></span>
              </button>
              <span className='ml-3 text-sm' id='reload-on-start'>
                <span className='font-medium text-gray-900'>Reload on start</span>
              </span>
            </div>
          </div>
        </div>
      ) : (
        <div className='flex min-h-0 flex-1 divide-x divide-gray-100'>
          {/* Initial prompt */}
          <div className='flex-1 overflow-y-auto px-6 py-4'>
            <h2 className='pb-2 text-base font-semibold text-gray-900'>Visited Pages</h2>
            <div className='flex flex-col gap-6'>
              {filteredOriginCallCounts.map(([origin, callCount]) => (
                <button key={origin} onClick={() => void setActiveOrigin(origin)} className='relative flex gap-x-4'>
                  <div className='absolute -bottom-6 left-0 top-0 flex w-6 justify-center'>
                    <div className='w-px bg-gray-200'></div>
                  </div>
                  <div className='relative flex size-6 flex-none items-center justify-center bg-white'>
                    <div className='size-1.5 rounded-full bg-gray-100 ring-1 ring-gray-300'></div>
                  </div>
                  <p className='flex-1 py-0.5 text-start text-xs leading-5 text-gray-500'>
                    <span className='font-medium text-gray-900'>{origin} </span>
                    {callCount} calls
                  </p>
                  <svg
                    xmlns='http://www.w3.org/2000/svg'
                    className='size-5 text-gray-900'
                    stroke='currentColor'
                    viewBox='0 0 24 24'
                    fill='none'
                  >
                    <path
                      d='M9 5L14.15 10C14.4237 10.2563 14.6419 10.5659 14.791 10.9099C14.9402 11.2539 15.0171 11.625 15.0171 12C15.0171 12.375 14.9402 12.7458 14.791 13.0898C14.6419 13.4339 14.4237 13.7437 14.15 14L9 19'
                      strokeWidth='2'
                      strokeLinecap='round'
                      strokeLinejoin='round'
                    />
                  </svg>
                </button>
              ))}
            </div>
          </div>

          {/* Active origin */}
          {activeOrigin && (
            <div className='flex-1 overflow-y-auto'>
              <h2 className='p-4 text-base font-semibold text-gray-900'>API calls on {activeOrigin}</h2>

              {calls
                .filter((_) => new URL(_.url).origin === activeOrigin)
                .map((call, index) => (
                  <div key={index} className='flex items-center justify-between p-4 even:bg-gray-50'>
                    <div className='flex min-w-0 flex-1 items-center text-sm leading-6'>
                      <span className='inline-flex items-center rounded-md bg-yellow-50 px-2 py-1 text-xs font-medium text-yellow-800 ring-1 ring-inset ring-yellow-600/20'>
                        {call.method}
                      </span>
                      <label
                        htmlFor={`comments-${index.toString()}`}
                        className='ml-2 inline-flex text-sm font-semibold leading-6 text-gray-900'
                        style={{
                          // TODO: replace with a Tailwind class once upstream issue is solved
                          // https://github.com/tailwindlabs/tailwindcss/discussions/2213
                          overflowWrap: 'anywhere',
                        }}
                      >
                        {
                          // Prepend slashes with zero width spaces to allow for nicer word breaks in long URLs
                          // https://stackoverflow.com/a/24489931
                          new URL(call.url).pathname.replace(/\//g, '/\u200B')
                        }
                      </label>
                      <p className='ml-2 inline-flex text-xs leading-5 text-gray-500'>
                        <time>
                          {
                            // TODO: use a library to get a proper "X time ago" string
                            Date.now() - call.time
                          }
                        </time>
                      </p>
                    </div>
                    <div className='ml-3 flex items-center'>
                      <input
                        id={`comments-${index.toString()}`}
                        aria-describedby={`comments-description-${index.toString()}`}
                        name='comments'
                        type='checkbox'
                        className='size-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-600'
                      />
                    </div>
                  </div>
                ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
};

export default PopupPage;
