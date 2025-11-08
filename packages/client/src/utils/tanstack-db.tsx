import { Context, createLiveQueryCollection, InitialQueryBuilder, QueryBuilder } from '@tanstack/react-db';

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
