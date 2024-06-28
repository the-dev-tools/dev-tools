import { Schema } from '@effect/schema';
import { Array, Effect, flow, Option, pipe, Struct } from 'effect';
import * as React from 'react';

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

export const useHosts = () => {
  const [hosts, setHosts] = React.useState<readonly Postman.Item[]>([]);

  const [collection] = PlasmoStorage.useStorage<typeof Postman.Collection.Encoded>({
    instance: Storage.Local,
    key: CollectionTag,
  });

  React.useEffect(
    () =>
      void Effect.gen(function* () {
        if (!collection) return;
        const { item } = yield* Schema.decode(Postman.Collection)(collection);
        setHosts(item);
      }).pipe(Effect.ignore, Runtime.runPromise),
    [collection],
  );

  return hosts;
};

export const addNavigation = (tab: chrome.tabs.Tab) =>
  Effect.gen(function* () {
    if (!tab.url) return;
    const url = yield* Utils.URL.make(tab.url);

    let collection = yield* getCollection;

    let host = Array.last(collection.item).pipe(Option.getOrUndefined);
    if (host?.name !== url.host) {
      host = Postman.Item.make({ name: url.host, item: [] });
    } else {
      collection = Struct.evolve(collection, { item: (_) => Array.dropRight(_, 1) });
    }

    host = Struct.evolve(host, {
      item: (_) => Array.append(_ ?? [], Postman.Item.make({ name: url.pathname, item: [] })),
    });

    yield* pipe(collection, Struct.evolve({ item: (_) => Array.append(_, host) }), setCollection);
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
