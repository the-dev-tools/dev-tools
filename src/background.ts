import type { ProtocolMapping } from 'devtools-protocol/types/protocol-mapping';
import { Effect, Option } from 'effect';

import * as Recorder from '@/recorder';
import { Runtime } from '@/runtime';

const sendDebuggerCommand = <Command extends keyof ProtocolMapping.Commands>(
  target: chrome.debugger.Debuggee,
  method: Command,
  ...commandParams: ProtocolMapping.Commands[Command]['paramsType']
) =>
  Effect.tryPromise<ProtocolMapping.Commands[Command]['returnType']>(() =>
    chrome.debugger.sendCommand(target, method, ...commandParams),
  );

const isDebuggerEvent = <Method extends keyof ProtocolMapping.Events>(
  match: Method,
  method: string,
  _params: unknown,
): _params is ProtocolMapping.Events[Method][0] => match === method;

// Debugger control
Recorder.watch({
  onStart: (tabId) =>
    Effect.gen(function* () {
      yield* Effect.tryPromise(() => chrome.debugger.attach({ tabId }, '1.0'));
      yield* sendDebuggerCommand({ tabId }, 'Network.enable');
    }).pipe(Effect.ignoreLogged),
  onStop: (tabId) =>
    Effect.gen(function* () {
      yield* sendDebuggerCommand({ tabId }, 'Network.disable');
      yield* Effect.tryPromise(() => chrome.debugger.detach({ tabId }));
    }).pipe(Effect.ignoreLogged),
});

// URL updates
chrome.tabs.onUpdated.addListener((tabId, { url }, tab) =>
  Effect.gen(function* () {
    if (url === undefined) return;
    const recorderTabId = yield* Recorder.getTabId;
    if (!Option.contains(recorderTabId, tabId)) return;
    yield* Recorder.addNavigation(tab);
  }).pipe(Effect.ignoreLogged, Runtime.runPromise),
);

// Debugger events
chrome.debugger.onEvent.addListener((source, method, params) =>
  Effect.gen(function* () {
    const recorderTabId = yield* Recorder.getTabId;
    if (!Option.contains(recorderTabId, source.tabId)) return;

    // Request
    if (isDebuggerEvent('Network.requestWillBeSent', method, params)) {
      if (params.type !== 'XHR') return;

      const data = yield* sendDebuggerCommand(source, 'Network.getRequestPostData', {
        requestId: params.requestId,
      }).pipe(Effect.catchAll(() => Effect.succeed(undefined)));

      yield* Recorder.addRequest(params, data);
    }

    // Response
    if (isDebuggerEvent('Network.responseReceived', method, params)) {
      if (params.type !== 'XHR') return;

      const body = yield* sendDebuggerCommand(source, 'Network.getResponseBody', { requestId: params.requestId }).pipe(
        Effect.catchAll(() => Effect.succeed(undefined)),
      );

      yield* Recorder.addResponse(params, body);
    }
  }).pipe(Effect.ignoreLogged, Runtime.runPromise),
);
