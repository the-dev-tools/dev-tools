import { create } from '@bufbuild/protobuf';
import { Endpoint, schema } from '@data-client/endpoint';
import { Array, Equivalence, Match, Option, pipe, Record } from 'effect';
import { EnvironmentMoveRequestSchema, EnvironmentService } from '../dist/buf/typescript/environment/v1/environment_pb';
import { MovePosition } from '../dist/buf/typescript/resources/v1/resources_pb';
import { EnvironmentEntity } from '../dist/meta/environment/v1/environment.entities';
import { MakeEndpointProps } from './resource';
import { createMethodKeyRecord, EndpointProps, makeEndpointFn, makeKey } from './utils';

export const move = ({ method, name }: MakeEndpointProps<typeof EnvironmentService.method.environmentMove>) => {
  // TODO: split version spec from example and simplify list schema
  const argsKey = (props: EndpointProps<typeof EnvironmentService.method.environmentMove> | null) => {
    if (props === null) return {};
    const { input, transport } = props;
    return createMethodKeyRecord(transport, method, input, ['workspaceId']);
  };

  const createCollectionFilter =
    ({ input, transport }: EndpointProps<typeof EnvironmentService.method.environmentMove>) =>
    (collectionKey: Record<string, string>) => {
      const argsKey = createMethodKeyRecord(transport, method, input, ['workspaceId']);
      const compare = Record.getEquivalence(Equivalence.string);
      return compare(argsKey, collectionKey);
    };

  const environmentListSchema = new schema.Collection([EnvironmentEntity], { argsKey, createCollectionFilter });

  const endpointFn = async (props: EndpointProps<typeof EnvironmentService.method.environmentMove>) => {
    await makeEndpointFn(method)(props);

    const snapshot = props.controller().snapshot(props.controller().getState());

    // TODO: implement a generic move helper
    return Option.gen(function* () {
      const Environments = yield* Option.fromNullable(snapshot.get(environmentListSchema, props));

      const { environmentId, position, targetEnvironmentId } = create(EnvironmentMoveRequestSchema, props.input);

      const offset = yield* pipe(
        Match.value(position),
        Match.when(MovePosition.AFTER, () => 1),
        Match.when(MovePosition.BEFORE, () => 0),
        Match.option,
      );

      const { move = [], rest = [] } = Array.groupBy(Environments, (_) =>
        _.environmentId.toString() === environmentId.toString() ? 'move' : 'rest',
      );

      const index = yield* Array.findFirstIndex(
        rest,
        (_) => _.environmentId.toString() === targetEnvironmentId.toString(),
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
    schema: { items: environmentListSchema },
    sideEffect: true,
  });
};
