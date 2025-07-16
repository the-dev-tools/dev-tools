import { create } from '@bufbuild/protobuf';
import { Endpoint } from '@data-client/endpoint';
import { Array, Match, Option, pipe } from 'effect';
import {
  BodyFormMoveRequestSchema,
  BodyService
} from '../dist/buf/typescript/collection/item/body/v1/body_pb';
import { MovePosition } from '../dist/buf/typescript/resources/v1/resources_pb';
import {
  BodyFormListItemEntity
} from '../dist/meta/collection/item/body/v1/body.entities';
import { MakeEndpointProps } from './resource';
import { EndpointProps, makeEndpointFn, makeKey, makeListCollection } from './utils';

export const moveBodyForm = ({ method, name }: MakeEndpointProps<typeof BodyService.method.bodyFormMove>) => {
  const list = makeListCollection({ inputPrimaryKeys: ['exampleId'], itemSchema: BodyFormListItemEntity, method });

  const endpointFn = async (props: EndpointProps<typeof BodyService.method.bodyFormMove>) => {
    await makeEndpointFn(method)(props);

    const snapshot = props.controller().snapshot(props.controller().getState());

    // TODO: implement a generic move helper
    return Option.gen(function* () {
      const variables = yield* Option.fromNullable(snapshot.get(list, props));

      const { bodyId, position, targetBodyId } = create(BodyFormMoveRequestSchema, props.input);

      const offset = yield* pipe(
        Match.value(position),
        Match.when(MovePosition.AFTER, () => 1),
        Match.when(MovePosition.BEFORE, () => 0),
        Match.option,
      );

      const { move = [], rest = [] } = Array.groupBy(variables, (_) =>
        _.bodyId.toString() === bodyId.toString() ? 'move' : 'rest',
      );

      const index = yield* Array.findFirstIndex(rest, (_) => _.bodyId.toString() === targetBodyId.toString());

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
