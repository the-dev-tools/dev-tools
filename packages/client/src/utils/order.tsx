import { Collection, gt, lt } from '@tanstack/react-db';
import { Array, Option, pipe, Predicate } from 'effect';
import { DroppableCollectionReorderEvent } from 'react-aria-components';
import { Protobuf } from '~/api';
import { queryCollection } from './tanstack-db';

interface OrderableItem {
  order: number;
}

export const handleCollectionReorder =
  <T extends OrderableItem>(
    collection: Collection<
      T,
      string,
      {
        getKeyObject: (input: T) => Partial<T>;
        update: (input: OrderableItem) => void;
      }
    >,
  ) =>
  async ({ keys, target: { dropPosition, key } }: DroppableCollectionReorderEvent): Promise<void> => {
    if (dropPosition === 'on') return;

    if (keys.size !== 1) return;

    const source = pipe(
      Array.fromIterable(keys),
      Array.head,
      Option.filter(Predicate.isString),
      Option.flatMapNullable((_) => collection.get(_)),
      Option.getOrNull,
    );

    const target = pipe(
      Option.liftPredicate(key, Predicate.isString),
      Option.flatMapNullable((_) => collection.get(_)),
      Option.getOrNull,
    );

    if (!source || !target || source === target) return;

    if (dropPosition === 'before') {
      const beforeTargetOrder = pipe(
        await queryCollection((_) =>
          _.from({ item: collection })
            .where((_) => lt(_.item?.order, target.order))
            .orderBy((_) => _.item?.order, 'desc')
            .select((_) => ({ order: _.item?.order }))
            .limit(1)
            .findOne(),
        ),
        Array.head,
        Option.map((_) => _.order as number),
        Option.getOrElse(() => Protobuf.MAX_FLOAT * -1),
      );
      const newOrder = target.order - (target.order - beforeTargetOrder) / 2;
      collection.utils.update({ ...collection.utils.getKeyObject(source), order: newOrder });
    }

    if (dropPosition === 'after') {
      const afterTargetOrder = pipe(
        await queryCollection((_) =>
          _.from({ item: collection })
            .where((_) => gt(_.item?.order, target.order))
            .orderBy((_) => _.item?.order)
            .select((_) => ({ order: _.item?.order }))
            .limit(1)
            .findOne(),
        ),
        Array.head,
        Option.map((_) => _.order as number),
        Option.getOrElse(() => Protobuf.MAX_FLOAT),
      );
      const newOrder = target.order + (afterTargetOrder - target.order) / 2;
      collection.utils.update({ ...collection.utils.getKeyObject(source), order: newOrder });
    }
  };

export const getNextOrder = async <T extends OrderableItem>(collection: Collection<T, string>): Promise<number> => {
  const lastOrder = pipe(
    await queryCollection((_) =>
      _.from({ item: collection })
        .orderBy((_) => _.item?.order, 'desc')
        .select((_) => ({ order: _.item?.order }))
        .limit(1)
        .findOne(),
    ),
    Array.head,
    Option.map((_) => _.order as number),
    Option.getOrElse(() => 0),
  );

  return lastOrder + (Protobuf.MAX_FLOAT - lastOrder) / 2;
};
