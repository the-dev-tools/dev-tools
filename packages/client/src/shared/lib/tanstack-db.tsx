import {
  and,
  Context,
  createLiveQueryCollection,
  eq,
  InitialQueryBuilder,
  QueryBuilder,
  Ref,
} from '@tanstack/react-db';
import { Array, pipe, Record } from 'effect';

export const pick = <T extends object, K extends (keyof T)[]>(s: T, ...keys: K) => {
  const out: Partial<T> = {};
  for (const k of keys) out[k] = s[k];
  return out as Pick<T, K[number]>;
};

export const queryCollection = async <TContext extends Context>(
  query: (q: InitialQueryBuilder) => QueryBuilder<TContext>,
) => {
  const liveQueryCollection = createLiveQueryCollection(query);
  await liveQueryCollection.preload();
  return [...liveQueryCollection.values()];
};

type BooleanExpression = ReturnType<typeof eq>;

export const eqStruct =
  <T extends object>(value: T) =>
  ({ item }: { item: Ref<T> }): BooleanExpression => {
    const eqs = pipe(
      Record.keys(value),
      Array.map((key) => eq(value[key], item[key])),
    );

    if (eqs.length === 0) return eq(true, false);
    if (eqs.length === 1) return eqs[0]!;
    return and(...(eqs as [BooleanExpression, BooleanExpression, ...BooleanExpression[]]));
  };
