import { create } from '@bufbuild/protobuf';
import { schema } from '@data-client/endpoint';
import { Array, Equivalence, Match, Option, pipe, Record } from 'effect';
import { MovePosition } from '../dist/buf/typescript/resources/v1/resources_pb';
import { WorkspaceMoveRequestSchema, WorkspaceService } from '../dist/buf/typescript/workspace/v1/workspace_pb';
import { WorkspaceEntity } from '../dist/meta/workspace/v1/workspace.entities';
import { MakeEndpointProps } from './resource';
import { createMethodKeyRecord, Endpoint, EndpointProps, makeEndpointFn, makeKey } from './utils';

export const move = ({ method, name }: MakeEndpointProps<typeof WorkspaceService.method.workspaceMove>) => {
  // TODO: split version spec from example and simplify list schema
  const argsKey = (props: EndpointProps<typeof WorkspaceService.method.workspaceMove> | null) => {
    if (props === null) return {};
    const { input, transport } = props;
    return createMethodKeyRecord(transport, method, input, []);
  };

  const createCollectionFilter =
    ({ input, transport }: EndpointProps<typeof WorkspaceService.method.workspaceMove>) =>
    (collectionKey: Record<string, string>) => {
      const argsKey = createMethodKeyRecord(transport, method, input, []);
      const compare = Record.getEquivalence(Equivalence.string);
      return compare(argsKey, collectionKey);
    };

  const workspaceListSchema = new schema.Collection([WorkspaceEntity], { argsKey, createCollectionFilter });

  const endpointFn = async (props: EndpointProps<typeof WorkspaceService.method.workspaceMove>) => {
    await makeEndpointFn(method)(props);

    const snapshot = props.controller().snapshot(props.controller().getState());

    // TODO: implement a generic move helper
    return Option.gen(function* () {
      const workspaces = yield* Option.fromNullable(snapshot.get(workspaceListSchema, props));

      const { position, targetWorkspaceId, workspaceId } = create(WorkspaceMoveRequestSchema, props.input);

      const offset = yield* pipe(
        Match.value(position),
        Match.when(MovePosition.AFTER, () => 1),
        Match.when(MovePosition.BEFORE, () => 0),
        Match.option,
      );

      const { move = [], rest = [] } = Array.groupBy(workspaces, (_) =>
        _.workspaceId.toString() === workspaceId.toString() ? 'move' : 'rest',
      );

      const index = yield* Array.findFirstIndex(rest, (_) => _.workspaceId.toString() === targetWorkspaceId.toString());

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
    schema: { items: workspaceListSchema },
    sideEffect: true,
  });
};
