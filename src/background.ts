import type { Protocol } from 'devtools-protocol';
import type { ProtocolMapping } from 'devtools-protocol/types/protocol-mapping';
import { Array, Effect, flow, Option, Predicate, String, Struct } from 'effect';

import * as Recorder from '@/recorder';
import { Runtime } from '@/runtime';

// PlasmoHQ implements a workaround to keep the background service worker alive
// in Chrome Extension Manifest V3, so doing it manually is not needed (for now)
// https://github.com/PlasmoHQ/plasmo/tree/main/api/persistent
// https://stackoverflow.com/questions/66618136/persistent-service-worker-in-chrome-extension

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

const resourceTypes = ['XHR', 'Fetch'] as const satisfies Protocol.Network.ResourceType[];

void Effect.gen(function* () {
  let collection = yield* Recorder.getCollection;
  const indexMap = Recorder.makeIndexMap();

  // Debugger control
  Recorder.watch({
    onStart: (tabId) =>
      Effect.gen(function* () {
        yield* Effect.tryPromise(() => chrome.debugger.attach({ tabId }, '1.0'));
        yield* sendDebuggerCommand({ tabId }, 'Network.enable');

        const tab = yield* Effect.tryPromise(() => chrome.tabs.get(tabId));
        collection = yield* Recorder.addNavigation(collection, tab);
      }).pipe(
        Effect.catchIf(flow(Struct.get('message'), String.startsWith('Cannot access')), () => Recorder.stop),
        Effect.ignoreLogged,
      ),
    onStop: (tabId) =>
      Effect.gen(function* () {
        yield* sendDebuggerCommand({ tabId }, 'Network.disable');
        yield* Effect.tryPromise(() => chrome.debugger.detach({ tabId }));
      }).pipe(
        Effect.catchIf(
          flow(
            Struct.get('message'),
            Predicate.some([
              String.startsWith('Debugger is not attached'),
              String.startsWith('No tab with given id'),
              String.startsWith('Cannot access'),
            ]),
          ),
          () => Effect.void,
        ),
        Effect.ignoreLogged,
      ),
    onReset: Effect.gen(function* () {
      collection = yield* Recorder.reset(indexMap);
    }).pipe(Effect.ignoreLogged),
  });

  // URL updates
  chrome.tabs.onUpdated.addListener((tabId, { url }, tab) =>
    Effect.gen(function* () {
      if (url === undefined) return;
      const recorderTabId = yield* Recorder.getTabId;
      if (!Option.contains(recorderTabId, tabId)) return;
      collection = yield* Recorder.addNavigation(collection, tab);
    }).pipe(Effect.ignoreLogged, Runtime.runPromise),
  );

  // Stop recording on debugger detach
  chrome.debugger.onDetach.addListener((source) =>
    Effect.gen(function* () {
      const recorderTabId = yield* Recorder.getTabId;
      if (!Option.contains(recorderTabId, source.tabId)) return;
      yield* Recorder.stop;
    }).pipe(Effect.ignoreLogged, Runtime.runPromise),
  );

  // Debugger events
  chrome.debugger.onEvent.addListener((source, method, params) =>
    Effect.gen(function* () {
      const recorderTabId = yield* Recorder.getTabId;
      if (!Option.contains(recorderTabId, source.tabId)) return;

      // Request
      if (isDebuggerEvent('Network.requestWillBeSent', method, params)) {
        if (!Array.contains(resourceTypes, params.type)) return;
        const { requestId } = params;

        const data = yield* sendDebuggerCommand(source, 'Network.getRequestPostData', { requestId }).pipe(
          Effect.catchAll(() => Effect.succeed(undefined)),
        );

        collection = yield* Recorder.addRequest(collection, indexMap, params, data);
      }

      // Response
      if (isDebuggerEvent('Network.responseReceived', method, params)) {
        if (!Array.contains(resourceTypes, params.type)) return;
        const { requestId } = params;

        const body = yield* sendDebuggerCommand(source, 'Network.getResponseBody', { requestId }).pipe(
          Effect.catchAll(() => Effect.succeed(undefined)),
        );

        collection = yield* Recorder.addResponse(collection, indexMap, params, body);
      }
    }).pipe(Effect.ignoreLogged, Runtime.runPromise),
  );

  // Sync collection
  yield* Effect.gen(function* () {
    yield* Effect.sleep('1 second');
    yield* Recorder.setCollection(collection);
  }).pipe(Effect.forever);
}).pipe(Effect.ignoreLogged, Runtime.runPromise);
