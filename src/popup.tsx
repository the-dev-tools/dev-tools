import * as React from 'react';

import { useStorage } from '@plasmohq/storage/hook';

import * as Storage from '@/storage';

import './style.css';

const PopupPageNew = () => {
  const [selectedHostname, setSelectedHostname] = React.useState<null | string>(null);
  const [hostnameSearchTerm, setHostnameSearchTerm] = React.useState('');

  const [recordingTabId, setRecordingTabId] = useStorage<number | null>(
    { instance: Storage.Local, key: Storage.RECORDING_TAB_ID },
    (_) => _ ?? null,
  );

  const [requests, setRequests] = useStorage<Storage.NetworkCall[]>(
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
    await setRequests([]);
  };

  const hostnames = Object.entries(
    requests.reduce<Record<string, number>>((result, value) => {
      const { hostname } = new URL(value.url);
      if (!result[hostname]) result[hostname] = 0;
      result[hostname]++;
      return result;
    }, {}),
  );

  const filteredHostnames = hostnameSearchTerm
    ? hostnames.filter(([hostname]) => hostname.includes(hostnameSearchTerm))
    : hostnames;

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
          value={hostnameSearchTerm}
          onChange={(event) => void setHostnameSearchTerm(event.target.value)}
        />
      </div>

      <div className='flex h-full divide-x divide-black'>
        <div className='flex flex-1 flex-col items-start gap-2 overflow-y-auto p-2'>
          <h2 className='font-bold'>Hostnames</h2>

          {filteredHostnames.map(([hostname, count]) => (
            <button key={hostname} onClick={() => void setSelectedHostname(hostname)}>
              {hostname} - {count} requests
            </button>
          ))}
        </div>

        <div className='flex flex-1 flex-col items-start gap-2 overflow-y-auto p-2'>
          <h2 className='font-bold'>Requests</h2>

          {selectedHostname === null && <p>Select a hostname to see the associated requests</p>}

          {requests
            .filter((_) => new URL(_.url).hostname === selectedHostname)
            .map((request, index) => (
              <div key={index.toString() + request.url}>
                <input type='checkbox' className='inline-block' /> {request.method}
                {' - '}
                <time>
                  {
                    // TODO: use a library to get a proper "X time ago" string
                    Date.now() - request.time
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
                    new URL(request.url).pathname.replace(/\//g, '/\u200B')
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
