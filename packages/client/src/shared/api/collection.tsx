import { Atom, useAtomSuspense } from '@effect-atom/atom-react';
import { Effect, HashMap } from 'effect';
import { runtimeAtom } from '../lib/runtime';
import { ApiCollection, ApiCollections, ApiCollectionSchema } from './collection.internal';

export * from './collection.internal';

export const getApiCollection = Effect.fn(function* <TSchema extends ApiCollectionSchema>(schema: TSchema) {
  const collectionMap = yield* ApiCollections;
  const collection = yield* HashMap.get(collectionMap, schema);
  return collection as unknown as ApiCollection<TSchema>;
});

const apiCollectionAtomFamily = Atom.family(<TSchema extends ApiCollectionSchema>(schema: TSchema) =>
  runtimeAtom.atom(getApiCollection(schema)),
);

export const useApiCollection = <TSchema extends ApiCollectionSchema>(schema: TSchema) =>
  useAtomSuspense(apiCollectionAtomFamily(schema)).value;
