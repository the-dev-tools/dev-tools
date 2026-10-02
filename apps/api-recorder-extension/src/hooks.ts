import { Effect, Option, pipe, Schema } from 'effect';
import * as React from 'react';

import * as Postman from '~postman';
import * as Recorder from '~recorder';
import { Runtime } from '~runtime';
import { type Storage } from '~storage';
import * as StorageModule from '~storage';

// React bindings for extension storage, used by the popup and tab pages only
// (kept separate so the background service worker doesn't bundle React)

/** React hook returning the current (encoded) value of a storage key, kept in sync with changes */
export const useStorageValue = <T>(storage: Storage, key: string) => {
  const [value, setValue] = React.useState<T | undefined>(undefined);

  React.useEffect(() => {
    let active = true;
    void storage.get<T>(key).then((_) => {
      if (active) setValue(_);
    });
    const unwatch = storage.watch({
      [key]: ({ newValue }) => void setValue(newValue as T | undefined),
    });
    return () => {
      active = false;
      unwatch();
    };
  }, [storage, key]);

  const set = React.useCallback(
    async (next: T) => {
      setValue(next);
      await storage.set(key, next);
    },
    [storage, key],
  );

  return [value, set] as const;
};

export const useState = <A, I>(storage: Storage, key: string, schema: Schema.Schema<A, I>) => {
  const [state, setState] = React.useState(Option.none<A>());
  const [stateEncoded, setStateEncoded] = useStorageValue<typeof schema.Encoded>(storage, key);

  React.useEffect(
    () =>
      void Effect.gen(function* () {
        if (stateEncoded === undefined) return;
        const state = yield* Schema.decode(schema)(stateEncoded);
        setState(Option.some(state));
      }).pipe(Effect.ignoreLogged, Runtime.runPromise),
    [schema, stateEncoded],
  );

  const set = (value: A) =>
    pipe(
      Schema.encode(schema)(value),
      Effect.flatMap((_) => Effect.tryPromise(() => setStateEncoded(_))),
    );

  return [state, set] as const;
};

export const useCollection = () => {
  const [collection, setCollection] = React.useState(new Postman.Collection());
  const [collectionEncoded] = useStorageValue<typeof Postman.Collection.Encoded>(
    StorageModule.Local,
    Recorder.CollectionTag,
  );

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

export const useTabId = () => {
  const [tabIdEncoded] = useStorageValue<typeof Recorder.TabId.Encoded>(StorageModule.Local, Recorder.TabIdTag);
  if (!tabIdEncoded) return Option.none();
  return Schema.decodeSync(Recorder.TabId)(tabIdEncoded);
};
