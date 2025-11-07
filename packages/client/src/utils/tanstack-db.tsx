import { Context, createLiveQueryCollection, InitialQueryBuilder, QueryBuilder } from '@tanstack/react-db';

export const queryCollection = <TContext extends Context>(
  query: (q: InitialQueryBuilder) => QueryBuilder<TContext>,
) => {
  const liveQueryCollection = createLiveQueryCollection(query);
  liveQueryCollection.startSyncImmediate();
  return [...liveQueryCollection.values()];
};
