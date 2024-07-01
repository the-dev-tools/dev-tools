import { Schema } from '@effect/schema';
import type Protocol from 'devtools-protocol';
import { Array, Effect, flow, MutableHashMap, Option, pipe, Struct } from 'effect';
import * as React from 'react';
import * as Uuid from 'uuid';

import * as PlasmoStorage from '@plasmohq/storage/hook';

import * as Postman from '@/postman';
import { Runtime } from '@/runtime';
import * as Storage from '@/storage';
import * as Utils from '@/utils';

const CollectionTag = 'Collection';

const getCollection = pipe(
  Effect.tryPromise(() => Storage.Local.get<typeof Postman.Collection.Encoded>(CollectionTag)),
  Effect.flatMap(
    flow(
      Option.fromNullable,
      Option.match({
        onNone: () => Effect.succeed(new Postman.Collection()),
        onSome: Schema.decode(Postman.Collection),
      }),
    ),
  ),
);

const setCollection = (collection: typeof Postman.Collection.Type) =>
  pipe(
    collection,
    Schema.encode(Postman.Collection),
    Effect.flatMap((_) => Effect.tryPromise(() => Storage.Local.set(CollectionTag, _))),
  );

export const useNavigations = () => {
  const [navigations, setNavigations] = React.useState<readonly Postman.Item[]>([]);

  const [collection] = PlasmoStorage.useStorage<typeof Postman.Collection.Encoded>({
    instance: Storage.Local,
    key: CollectionTag,
  });

  React.useEffect(
    () =>
      void Effect.gen(function* () {
        if (!collection) return;
        const { item } = yield* Schema.decode(Postman.Collection)(collection);
        setNavigations(item);
      }).pipe(Effect.ignore, Runtime.runPromise),
    [collection],
  );

  return navigations;
};

export const addNavigation = (tab: chrome.tabs.Tab) =>
  Effect.gen(function* () {
    if (!tab.url) return;
    const url = yield* Utils.URL.make(tab.url);

    let collection = yield* getCollection;

    let host = Array.last(collection.item).pipe(Option.getOrUndefined);
    if (host?.name !== url.host) {
      host = Postman.Item.make({ id: Uuid.v4(), name: url.host, item: [] });
    } else {
      collection = Struct.evolve(collection, { item: (_) => Array.dropRight(_, 1) });
    }

    host = Struct.evolve(host, {
      item: (_) => Array.append(_ ?? [], Postman.Item.make({ id: Uuid.v4(), name: url.pathname, item: [] })),
    });

    yield* pipe(collection, Struct.evolve({ item: (_) => Array.append(_, host) }), setCollection);
  });

const requestIdIndexMap = MutableHashMap.make<[string, { host: number; navigation: number; request: number }][]>();

export const addRequest = (
  { requestId, request }: Protocol.Network.RequestWillBeSentEvent,
  { postData }: Partial<Protocol.Network.GetRequestPostDataResponse> = {},
) =>
  Effect.gen(function* () {
    let collection = yield* getCollection;
    let host = yield* Array.last(collection.item);
    let navigation = yield* pipe(host.item, Option.fromNullable, Option.flatMap(Array.last));

    const requestItem = new Postman.Item({
      id: requestId,
      name: request.url,
      request: new Postman.RequestClass({
        url: request.url,
        method: request.method,
        body: new Postman.Body({ raw: postData }),
      }),
    });

    navigation = Struct.evolve(navigation, { item: (_) => Array.append(_ ?? [], requestItem) });
    host = Struct.evolve(host, { item: (_) => pipe(_ ?? [], Array.dropRight(1), Array.append(navigation)) });
    collection = Struct.evolve(collection, { item: (_) => pipe(_, Array.dropRight(1), Array.append(host)) });

    MutableHashMap.set(requestIdIndexMap, requestId, {
      host: collection.item.length - 1,
      navigation: (host.item?.length ?? 0) - 1,
      request: (navigation.item?.length ?? 0) - 1,
    });

    yield* setCollection(collection);
  });

export const addResponse = (
  { requestId, response }: Protocol.Network.ResponseReceivedEvent,
  { body }: Partial<Protocol.Network.GetResponseBodyResponse> = {},
) =>
  Effect.gen(function* () {
    const index = yield* MutableHashMap.get(requestIdIndexMap, requestId);

    const collection = yield* getCollection;
    const host = yield* Array.get(collection.item, index.host);
    const navigation = yield* pipe(host.item, Option.fromNullable, Option.flatMap(Array.get(index.navigation)));
    const request = yield* pipe(navigation.item, Option.fromNullable, Option.flatMap(Array.get(index.request)));

    const responseItem = new Postman.Response({
      code: response.status,
      status: response.statusText,
      body,
    });

    const newRequest = new Postman.Item({ ...request, response: [responseItem] });
    const newNavigation = Struct.evolve(navigation, { item: (_) => Array.replace(_ ?? [], index.request, newRequest) });
    const newHost = Struct.evolve(host, { item: (_) => Array.replace(_ ?? [], index.navigation, newNavigation) });
    const newCollection = Struct.evolve(collection, { item: (_) => Array.replace(_, index.host, newHost) });

    MutableHashMap.remove(requestIdIndexMap, requestId);

    yield* setCollection(newCollection);
  });

const TabIdTag = 'TabId';
const TabId = Schema.Option(Schema.Number);

export const getTabId = Effect.gen(function* () {
  const tabId = yield* Effect.tryPromise(() => Storage.Local.get<typeof TabId.Encoded>(TabIdTag));
  if (!tabId) return Option.none();
  return yield* Schema.decode(TabId)(tabId);
});

export const useTabId = () => {
  const [tabIdEncoded] = PlasmoStorage.useStorage<typeof TabId.Encoded>({
    instance: Storage.Local,
    key: TabIdTag,
  });
  if (!tabIdEncoded) return Option.none();
  return Schema.decodeSync(TabId)(tabIdEncoded);
};

export const start = Effect.gen(function* () {
  const tabs = yield* Effect.tryPromise(() => chrome.tabs.query({ active: true, currentWindow: true }));
  const tab = tabs[0];
  if (!tab?.id) return;

  yield* addNavigation(tab);

  yield* pipe(
    tab.id,
    Option.some,
    Schema.encode(TabId),
    Effect.flatMap((_) => Effect.tryPromise(() => Storage.Local.set(TabIdTag, _))),
  );
});

export const stop = pipe(
  Option.none(),
  Schema.encode(TabId),
  Effect.flatMap((_) => Effect.tryPromise(() => Storage.Local.set(TabIdTag, _))),
);

export const reset = Effect.gen(function* () {
  yield* stop.pipe(Effect.ignoreLogged);
  yield* pipe(
    new Postman.Collection(),
    Schema.encode(Postman.Collection),
    Effect.flatMap((_) => Effect.tryPromise(() => Storage.Local.set(CollectionTag, _))),
  );
  MutableHashMap.clear(requestIdIndexMap);
});

interface WatchProps {
  onStart: (tabId: number) => Effect.Effect<void>;
  onStop: (tabId: number) => Effect.Effect<void>;
}

export const watch = ({ onStart, onStop }: WatchProps) =>
  Storage.Local.watch({
    [TabIdTag]: (_) =>
      void pipe(
        Schema.decodeUnknown(Storage.Change(TabId))(_),
        Effect.flatMap((_) =>
          Option.match(Option.flatten(_.newValue), {
            onSome: onStart,
            onNone: () => Effect.flatMap(Option.flatten(_.oldValue), onStop),
          }),
        ),
        Effect.ignoreLogged,
        Runtime.runPromise,
      ),
  });
