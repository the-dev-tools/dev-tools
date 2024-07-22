import { Schema } from '@effect/schema';
import * as Devtools from 'devtools-protocol';
import { Array, Effect, flow, MutableHashMap, Option, pipe, Record, Struct } from 'effect';
import * as React from 'react';
import * as Uuid from 'uuid';

import * as PlasmoStorage from '@plasmohq/storage/hook';

import * as Utils from '@the-dev-tools/utils';

import * as Postman from '~postman';
import { Runtime } from '~runtime';
import * as Storage from '~storage';

const CollectionTag = 'Collection';

export const getCollection = pipe(
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

export const setCollection = (collection: Postman.Collection) =>
  pipe(
    collection,
    Schema.encode(Postman.Collection),
    Effect.flatMap((_) => Effect.tryPromise(() => Storage.Local.set(CollectionTag, _))),
  );

export const useCollection = () => {
  const [collection, setCollection] = React.useState(new Postman.Collection());

  const [collectionEncoded] = PlasmoStorage.useStorage<typeof Postman.Collection.Encoded>({
    instance: Storage.Local,
    key: CollectionTag,
  });

  React.useEffect(
    () =>
      void Effect.gen(function* () {
        if (!collectionEncoded) return;
        const collection = yield* Schema.decode(Postman.Collection)(collectionEncoded);
        setCollection(collection);
      }).pipe(Effect.ignore, Runtime.runPromise),
    [collectionEncoded],
  );

  return collection;
};

export const addNavigation = (collection: Postman.Collection, tab: chrome.tabs.Tab) =>
  Effect.gen(function* () {
    if (!tab.url) return collection;
    const url = yield* Utils.URL.make(tab.url);

    let newCollection = collection;

    let host = Array.head(newCollection.item).pipe(Option.getOrUndefined);
    if (host?.name !== url.host) {
      host = Postman.Item.make({ id: Uuid.v4(), name: url.host, item: [] });
    } else {
      newCollection = Struct.evolve(newCollection, { item: (_) => Array.drop(_, 1) });
    }

    let pathname = Array.head(host.item ?? []).pipe(Option.getOrUndefined);
    if (pathname?.name !== url.pathname) {
      pathname = Postman.Item.make({ id: Uuid.v4(), name: url.pathname, item: [] });
    } else {
      host = Struct.evolve(host, { item: (_) => Array.drop(_ ?? [], 1) });
    }

    host = Struct.evolve(host, { item: (_) => Array.prepend(_ ?? [], pathname) });
    newCollection = Struct.evolve(newCollection, { item: (_) => Array.prepend(_, host) });

    return newCollection;
  });

export const makeIndexMap = () =>
  MutableHashMap.make<[string, { host: number; navigation: number; request: number }][]>();

const hostnameBlacklist = ['api-iam.intercom.io'];

export const addRequest = (
  collection: Postman.Collection,
  indexMap: ReturnType<typeof makeIndexMap>,
  { requestId, request, wallTime }: Devtools.Protocol.Network.RequestWillBeSentEvent,
  { postData }: Partial<Devtools.Protocol.Network.GetRequestPostDataResponse> = {},
) =>
  Effect.gen(function* () {
    const url = yield* Utils.URL.make(request.url);
    if (Array.contains(hostnameBlacklist, url.hostname)) return collection;

    const host = yield* Array.head(collection.item);
    const navigation = yield* pipe(host.item, Option.fromNullable, Option.flatMap(Array.head));

    const postBody = pipe(
      postData,
      Option.fromNullable,
      Option.map((_) => new Postman.Body({ mode: 'raw', raw: _ })),
    );

    const header = pipe(
      request.headers,
      Record.toEntries,
      Array.map(([key, value]) => new Postman.Header({ key, value })),
    );

    const timestampVariable = new Postman.Variable({
      key: 'timestamp',
      type: 'number',
      value: wallTime,
    });

    const requestItem = new Postman.Item({
      id: Uuid.v4(),
      name: request.url,
      variable: [timestampVariable],
      request: new Postman.RequestClass({
        url: request.url,
        method: request.method,
        body: Option.getOrNull(postBody),
        header,
      }),
    });

    const newNavigation = Struct.evolve(navigation, { item: (_) => Array.prepend(_ ?? [], requestItem) });
    const newHost = Struct.evolve(host, { item: (_) => pipe(_ ?? [], Array.drop(1), Array.prepend(newNavigation)) });
    const newCollection = Struct.evolve(collection, { item: (_) => pipe(_, Array.drop(1), Array.prepend(newHost)) });

    MutableHashMap.set(indexMap, requestId, {
      host: newCollection.item.length,
      navigation: newHost.item?.length ?? 0,
      request: newNavigation.item?.length ?? 0,
    });

    return newCollection;
  });

export const addResponse = (
  collection: Postman.Collection,
  indexMap: ReturnType<typeof makeIndexMap>,
  { requestId, response }: Devtools.Protocol.Network.ResponseReceivedEvent,
  { body }: Partial<Devtools.Protocol.Network.GetResponseBodyResponse> = {},
) =>
  Effect.gen(function* () {
    const url = yield* Utils.URL.make(response.url);
    if (Array.contains(hostnameBlacklist, url.hostname)) return collection;

    const index = yield* MutableHashMap.get(indexMap, requestId);

    const host = yield* Array.get(collection.item, collection.item.length - index.host);
    const navigation = yield* pipe(
      host.item,
      Option.fromNullable,
      Option.flatMap(Array.get((host.item?.length ?? 0) - index.navigation)),
    );
    const request = yield* pipe(
      navigation.item,
      Option.fromNullable,
      Option.flatMap(Array.get((navigation.item?.length ?? 0) - index.request)),
    );

    const header = pipe(
      response.headers,
      Record.toEntries,
      Array.map(([key, value]) => new Postman.Header({ key, value })),
    );

    const responseItem = new Postman.Response({
      code: response.status,
      status: response.statusText,
      body,
      header,
    });

    const newRequest = new Postman.Item({ ...request, response: [responseItem] });
    const newNavigation = Struct.evolve(navigation, {
      item: (_) => Array.replace(_ ?? [], (_?.length ?? 0) - index.request, newRequest),
    });
    const newHost = Struct.evolve(host, {
      item: (_) => Array.replace(_ ?? [], (_?.length ?? 0) - index.navigation, newNavigation),
    });
    const newCollection = Struct.evolve(collection, {
      item: (_) => Array.replace(_, _.length - index.host, newHost),
    });

    MutableHashMap.remove(indexMap, requestId);

    return newCollection;
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

const ResetRequestTag = 'ResetRequestTag';
const ResetRequest = Schema.Option(Schema.Boolean);

export const setReset = (reset: boolean) =>
  pipe(
    Option.some(reset),
    Schema.encode(ResetRequest),
    Effect.flatMap((_) => Effect.tryPromise(() => Storage.Local.set(ResetRequestTag, _))),
  );

export const reset = (indexMap: ReturnType<typeof makeIndexMap>) =>
  Effect.gen(function* () {
    const newCollection = new Postman.Collection();
    yield* stop.pipe(Effect.ignoreLogged);
    yield* pipe(
      newCollection,
      Schema.encode(Postman.Collection),
      Effect.flatMap((_) => Effect.tryPromise(() => Storage.Local.set(CollectionTag, _))),
    );
    yield* setReset(false);
    MutableHashMap.clear(indexMap);
    return newCollection;
  });

interface WatchProps {
  onStart: (tabId: number) => Effect.Effect<void>;
  onStop: (tabId: number) => Effect.Effect<void>;
  onReset: Effect.Effect<void>;
}

export const watch = ({ onStart, onStop, onReset }: WatchProps) =>
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
    [ResetRequestTag]: (_) =>
      void pipe(
        Schema.decodeUnknown(Storage.Change(ResetRequest))(_),
        Effect.flatMap((_) =>
          Option.match(Option.flatten(_.newValue), {
            onSome: () => onReset,
            onNone: () => Effect.void,
          }),
        ),
        Effect.ignoreLogged,
        Runtime.runPromise,
      ),
  });
