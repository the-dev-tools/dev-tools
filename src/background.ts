import type { ProtocolMapping } from 'devtools-protocol/types/protocol-mapping';

import * as Storage from '@/storage';

const sendDebuggerCommand = <Command extends keyof ProtocolMapping.Commands>(
  target: chrome.debugger.Debuggee,
  method: Command,
  ...commandParams: ProtocolMapping.Commands[Command]['paramsType']
) =>
  chrome.debugger.sendCommand(target, method, ...commandParams) as Promise<
    ProtocolMapping.Commands[Command]['returnType']
  >;

const matchDebuggerEvent = <Method extends keyof ProtocolMapping.Events>(
  match: Method,
  method: string,
  _params: unknown,
): _params is ProtocolMapping.Events[Method][0] => match === method;

// Toggle debugger on/off depending on state
Storage.Local.watch({
  [Storage.RECORDING_TAB_ID]: async (_) => {
    if (typeof _.oldValue === 'number') {
      const target = { tabId: _.oldValue };
      await sendDebuggerCommand(target, 'Network.disable').catch(() => undefined);
      await chrome.debugger.detach(target).catch(() => undefined);
    }

    if (typeof _.newValue !== 'number') return;

    const target = { tabId: _.newValue };
    await chrome.debugger.attach(target, '1.0');
    await sendDebuggerCommand(target, 'Network.enable');
  },
});

// Listen for debugger events on recording tab
chrome.debugger.onEvent.addListener(async (source: chrome.debugger.Debuggee, method: string, params: unknown) => {
  const recordingTabId = await Storage.Local.get<number | null>(Storage.RECORDING_TAB_ID);
  if (source.tabId !== recordingTabId) return;

  if (matchDebuggerEvent('Network.requestWillBeSent', method, params)) {
    if (params.type !== 'XHR') return;

    const data = await sendDebuggerCommand(source, 'Network.getRequestPostData', { requestId: params.requestId }).catch(
      () => undefined,
    );
    console.log('request', params, data);

    // TODO: save more response data

    const recordedCalls = (await Storage.Local.get<Storage.NetworkCall[]>(Storage.RECORDED_CALLS)) ?? [];

    await Storage.Local.set(Storage.RECORDED_CALLS, [
      ...recordedCalls,
      {
        method: params.request.method,
        url: params.request.url,
        time: params.timestamp,
      } satisfies Storage.NetworkCall,
    ]);
  }

  if (matchDebuggerEvent('Network.responseReceived', method, params)) {
    if (params.type !== 'XHR') return;

    const body = await sendDebuggerCommand(source, 'Network.getResponseBody', { requestId: params.requestId }).catch(
      () => undefined,
    );
    console.log('response', params, body);

    // TODO: save response data
  }
});
