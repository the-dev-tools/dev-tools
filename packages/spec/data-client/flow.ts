import { create } from '@bufbuild/protobuf';
import { Array, Match, Option, pipe } from 'effect';
import {
  Endpoint,
  EndpointProps,
  makeEndpointFn,
  makeKey,
  makeListCollection,
} from '@the-dev-tools/spec-lib/data-client/utils.ts';
import {
  FlowVariableMoveRequestSchema,
  FlowVariableService,
} from '../dist/buf/typescript/flow_variable/v1/flow_variable_pb';
import { MovePosition } from '../dist/buf/typescript/resource/v1/resource_pb';
import { FlowVariableListItemEntity } from '../dist/data-client/flow_variable/v1/flow_variable.entities';
import { MakeEndpointProps } from './resource';

export const moveVariable = ({
  method,
  name,
}: MakeEndpointProps<typeof FlowVariableService.method.flowVariableMove>) => {
  const list = makeListCollection({ inputPrimaryKeys: ['flowId'], itemSchema: FlowVariableListItemEntity, method });

  const endpointFn = async (props: EndpointProps<typeof FlowVariableService.method.flowVariableMove>) => {
    await makeEndpointFn(method)(props);

    const snapshot = props.controller().snapshot(props.controller().getState());

    // TODO: implement a generic move helper
    return Option.gen(function* () {
      const variables = yield* Option.fromNullable(snapshot.get(list, props));

      const { position, targetVariableId, variableId } = create(FlowVariableMoveRequestSchema, props.input);

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
