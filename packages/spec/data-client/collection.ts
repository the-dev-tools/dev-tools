import { create } from '@bufbuild/protobuf';
import { Endpoint } from '@data-client/endpoint';
import { Array, Match, Option, pipe } from 'effect';
import { CollectionMoveRequestSchema, CollectionService } from '../dist/buf/typescript/collection/v1/collection_pb';
import { MovePosition } from '../dist/buf/typescript/resources/v1/resources_pb';
import { CollectionListItemEntity } from '../dist/meta/collection/v1/collection.entities';
import { MakeEndpointProps } from './resource';
import { EndpointProps, makeEndpointFn, makeKey, makeListCollection } from './utils';

export const move = ({ method, name }: MakeEndpointProps<typeof CollectionService.method.collectionMove>) => {
  const list = makeListCollection({ inputPrimaryKeys: ['workspaceId'], itemSchema: CollectionListItemEntity, method });

  const endpointFn = async (props: EndpointProps<typeof CollectionService.method.collectionMove>) => {
    await makeEndpointFn(method)(props);

    const snapshot = props.controller().snapshot(props.controller().getState());

    // TODO: implement a generic move helper
    return Option.gen(function* () {
      const items = yield* Option.fromNullable(snapshot.get(list, props));

      const { collectionId, position, targetCollectionId } = create(CollectionMoveRequestSchema, props.input);

      const offset = yield* pipe(
        Match.value(position),
        Match.when(MovePosition.AFTER, () => 1),
        Match.when(MovePosition.BEFORE, () => 0),
        Match.option,
      );

      const { move = [], rest = [] } = Array.groupBy(items, (_) =>
        _.collectionId.toString() === collectionId.toString() ? 'move' : 'rest',
      );

      const index = yield* Array.findFirstIndex(
        rest,
        (_) => _.collectionId.toString() === targetCollectionId.toString(),
      );

      const [before, after] = Array.splitAt(rest, index + offset);

      return [...before, ...move, ...after];
    }).pipe(
      Option.match({
        onNone: () => ({}),
        onSome: (_) => ({ items: _ }),
      }),
    );
  };

  return new Endpoint(endpointFn, {
    key: makeKey(method, name),
    name,
    schema: { items: list },
    sideEffect: true,
  });
};
