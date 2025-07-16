import { create } from '@bufbuild/protobuf';
import { Endpoint } from '@data-client/endpoint';
import { Array, Match, Option, pipe } from 'effect';
import { MovePosition } from '../dist/buf/typescript/resources/v1/resources_pb';
import { VariableMoveRequestSchema, VariableService } from '../dist/buf/typescript/variable/v1/variable_pb';
import { VariableListItemEntity } from '../dist/meta/variable/v1/variable.entities';
import { MakeEndpointProps } from './resource';
import { EndpointProps, makeEndpointFn, makeKey, makeListCollection } from './utils';

export const move = ({ method, name }: MakeEndpointProps<typeof VariableService.method.variableMove>) => {
  const list = makeListCollection({ inputPrimaryKeys: ['environmentId'], itemSchema: VariableListItemEntity, method });

  const endpointFn = async (props: EndpointProps<typeof VariableService.method.variableMove>) => {
    await makeEndpointFn(method)(props);

    const snapshot = props.controller().snapshot(props.controller().getState());

    // TODO: implement a generic move helper
    return Option.gen(function* () {
      const variables = yield* Option.fromNullable(snapshot.get(list, props));

      const { position, targetVariableId, variableId } = create(VariableMoveRequestSchema, props.input);

      const offset = yield* pipe(
        Match.value(position),
        Match.when(MovePosition.AFTER, () => 1),
        Match.when(MovePosition.BEFORE, () => 0),
        Match.option,
      );

      const { move = [], rest = [] } = Array.groupBy(variables, (_) =>
        _.variableId.toString() === variableId.toString() ? 'move' : 'rest',
      );

      const index = yield* Array.findFirstIndex(rest, (_) => _.variableId.toString() === targetVariableId.toString());

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
