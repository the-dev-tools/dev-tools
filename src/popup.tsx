import { Effect, Option } from 'effect';

import * as Recorder from '@/recorder';
import { Runtime } from '@/runtime';

import './style.css';

const PopupPageNew = () => {
  const hosts = Recorder.useHosts();
  const tabId = Recorder.useTabId();

  return (
    <div className='flex h-[35rem] w-[50rem] flex-col divide-y divide-black'>
      <div className='flex gap-2 p-2'>
        <h1 className='font-bold'>API Recorder</h1>

        {Option.match(tabId, {
          onNone: () => (
            <button onClick={() => void Recorder.start.pipe(Effect.ignoreLogged, Runtime.runPromise)}>Start</button>
          ),
          onSome: () => (
            <button onClick={() => void Recorder.stop.pipe(Effect.ignoreLogged, Runtime.runPromise)}>Stop</button>
          ),
        })}

        <button onClick={() => void Recorder.reset.pipe(Effect.ignoreLogged, Runtime.runPromise)}>Reset</button>
      </div>

      <div className='flex min-h-0 flex-1 divide-x divide-black'>
        <div className='flex flex-1 flex-col items-start gap-2 overflow-y-auto p-2'>
          <h2 className='font-bold'>Hostnames</h2>

          {hosts.map((_, index) => (
            <div key={(_.name ?? '') + index.toString()}>
              <div className='font-bold'>{_.name}</div>
              {_.item?.map((_, index) => <div key={(_.name ?? '') + index.toString()}>{_.name}</div>)}
            </div>
          ))}
        </div>

        <div className='flex flex-1 flex-col items-start gap-2 overflow-y-auto p-2'>
          <h2 className='font-bold'>Requests</h2>

          <p>Select a hostname to see the associated requests</p>
        </div>
      </div>
    </div>
  );
};

export default PopupPageNew;
