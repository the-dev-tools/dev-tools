import type { Protocol } from 'devtools-protocol';
import type { ProtocolMapping } from 'devtools-protocol/types/protocol-mapping';

import { Array, Effect, flow, Option, Predicate, String, Struct } from 'effect';

import * as Recorder from '~recorder';
import { Runtime } from '~runtime';

// Since Chrome 118, an attached `chrome.debugger` session keeps the extension
// service worker alive, so no keep-alive workaround is needed while recording.
// When the worker does restart, the collection is reloaded from storage below.
// https://developer.chrome.com/docs/extensions/develop/concepts/service-workers/lifecycle

const sendDebuggerCommand = <Command extends keyof ProtocolMapping.Commands>(
  target: chrome.debugger.Debuggee,
  method: Command,
  ...commandParams: ProtocolMapping.Commands[Command]['paramsType']
) =>
  Effect.tryPromise(
    async () =>
      (await chrome.debugger.sendCommand(
        target,
        method,
        // CDP param types are closed interfaces; `@types/chrome` wants an index signature
        commandParams[0] as Record<string, unknown> | undefined,
      )) as ProtocolMapping.Commands[Command]['returnType'],
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
    onReset: Effect.gen(function* () {
      collection = yield* Recorder.reset(indexMap);
    }).pipe(Effect.ignoreLogged),
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
