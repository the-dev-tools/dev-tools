import { Schema } from '@effect/schema';
import { Effect, Option, pipe } from 'effect';
import * as React from 'react';

import * as PlasmoStorage from '@plasmohq/storage';
import * as PlasmoStorageHook from '@plasmohq/storage/hook';

import { Runtime } from '@/runtime';

export const Local = new PlasmoStorage.Storage({ area: 'local' });

export const Change = <S extends Schema.Schema.All>(schema: S) => {
  const value = pipe(schema, Schema.optional({ as: 'Option' }));
  return Schema.Struct({ newValue: value, oldValue: value });
};

export const get = <T>(storage: PlasmoStorage.Storage, key: string, schema: Schema.Schema<T>) =>
  Effect.gen(function* () {
    const value = yield* Effect.tryPromise(() => storage.get<typeof schema.Encoded>(key));
    if (value === undefined) return Option.none();
    return yield* Schema.decode(schema)(value).pipe(Effect.map(Option.some));
  });

export const set =
  <A, I>(storage: PlasmoStorage.Storage, key: string, schema: Schema.Schema<A, I>) =>
  (value: A) =>
    pipe(
      Schema.encode(schema)(value),
      Effect.flatMap((_) => Effect.tryPromise(() => storage.set(key, _))),
    );

export const useState = <A, I>(storage: PlasmoStorage.Storage, key: string, schema: Schema.Schema<A, I>) => {
  const [state, setState] = React.useState(Option.none<A>());
  const [stateEncoded, setStateEncoded] = PlasmoStorageHook.useStorage<typeof schema.Encoded>({
    instance: storage,
    key,
  });

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
