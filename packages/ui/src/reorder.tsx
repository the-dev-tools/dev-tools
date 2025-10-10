import { Array, Match, Option, pipe, Predicate } from 'effect';
import { ElementType } from 'react';
import { DroppableCollectionReorderEvent } from 'react-aria-components';
import { MovePosition } from '@the-dev-tools/spec/resource/v1/resource_pb';
import { tw } from './tailwind-literal';

interface BasicReorderCallbackProps {
  position: MovePosition;
  source: string;
  target: string;
}

export const basicReorder =
  (callback: (props: BasicReorderCallbackProps) => void) =>
  ({ keys, target: { dropPosition, key } }: DroppableCollectionReorderEvent) =>
    Option.gen(function* () {
      const position = yield* pipe(
        Match.value(dropPosition),
        Match.when('after', () => MovePosition.AFTER),
        Match.when('before', () => MovePosition.BEFORE),
        Match.option,
      );

      const source = yield* pipe(
        yield* Option.liftPredicate(keys, (_) => _.size === 1),
        Array.fromIterable,
        Array.head,
        Option.filter(Predicate.isString),
      );

      const target = yield* Option.liftPredicate(key, Predicate.isString);

      if (source === target) return;

      callback({ position, source, target });
    });

interface DropIndicator {
  as?: ElementType;
}

export const DropIndicatorHorizontal = ({ as: Component = 'div' }: DropIndicator) => (
  <Component className={tw`relative z-10 col-span-full h-0 w-full ring ring-violet-700`} />
);

export const DropIndicatorVertical = ({ as: Component = 'div' }: DropIndicator) => (
  <Component className={tw`relative z-10 row-span-full h-full w-0 ring ring-violet-700`} />
);
