import * as React from 'react';

import { useStorage } from '@plasmohq/storage/hook';

import * as Storage from '@/storage';

import './style.css';

const PopupPageNew = () => {
  const [activeOrigin, setActiveOrigin] = React.useState<null | string>(null);
  const [searchTerm, setSearchTerm] = React.useState('');

  const [recordingTabId, setRecordingTabId] = useStorage<number | null>(
    { instance: Storage.Local, key: Storage.RECORDING_TAB_ID },
    (_) => _ ?? null,
  );

  const [calls, setCalls] = useStorage<Storage.NetworkCall[]>(
    { instance: Storage.Local, key: Storage.RECORDED_CALLS },
    (_) => _ ?? [],
  );

  const startRecording = async () => {
    const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
    const tabId = tabs[0]?.id;
    if (!tabId) return;
    await setRecordingTabId(tabId);
  };

  const stopRecording = async () => {
    await setRecordingTabId(null);
    await setCalls([]);
  };

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
    <div className='h-[35rem] w-[50rem] divide-y divide-black'>
      <div className='flex gap-2 p-2'>
        <h1 className='font-bold'>API Recorder</h1>

        {recordingTabId === null ? (
          <button onClick={() => void startRecording()}>Start</button>
        ) : (
          <button onClick={() => void stopRecording()}>Stop</button>
        )}

        <input
          className='flex-1'
          type='text'
          placeholder='Search...'
          value={searchTerm}
          onChange={(event) => void setSearchTerm(event.target.value)}
        />
      </div>

      <div className='flex h-full divide-x divide-black'>
        <div className='flex flex-1 flex-col items-start gap-2 overflow-y-auto p-2'>
          <h2 className='font-bold'>Call origins</h2>

          {filteredOriginCallCounts.map(([origin, callCount]) => (
            <button key={origin} onClick={() => void setActiveOrigin(origin)}>
              {origin} - {callCount} calls
            </button>
          ))}
        </div>

        <div className='flex flex-1 flex-col items-start gap-2 overflow-y-auto p-2'>
          <h2 className='font-bold'>Calls</h2>

          {activeOrigin === null && <p>Select origin to see the calls</p>}

          {calls
            .filter((_) => new URL(_.url).origin === activeOrigin)
            .map((call, index) => (
              <div key={index.toString() + call.url}>
                <input type='checkbox' className='inline-block' /> {call.method}
                {' - '}
                <time>
                  {
                    // TODO: use a library to get a proper "X time ago" string
                    Date.now() - call.time
                  }
                </time>
                {' - '}
                <span
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
                </span>
              </div>
            ))}
        </div>
      </div>
    </div>
  );
};

export default PopupPageNew;
