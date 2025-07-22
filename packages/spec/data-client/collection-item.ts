import { create } from '@bufbuild/protobuf';
import { Endpoint, schema } from '@data-client/endpoint';
import { Array, Equivalence, Match, Option, pipe, Record, Struct } from 'effect';
import { EndpointService } from '../dist/buf/typescript/collection/item/endpoint/v1/endpoint_pb';
import {
  ExampleMoveRequestSchema,
  ExampleSchema,
  ExampleService,
} from '../dist/buf/typescript/collection/item/example/v1/example_pb';
import { FolderService } from '../dist/buf/typescript/collection/item/folder/v1/folder_pb';
import {
  CollectionItem,
  CollectionItemListRequest,
  CollectionItemMoveRequestSchema,
  CollectionItemSchema,
  CollectionItemService,
  ItemKind,
} from '../dist/buf/typescript/collection/item/v1/item_pb';
import { MovePosition } from '../dist/buf/typescript/resources/v1/resources_pb';
import { EndpointListItemEntity } from '../dist/meta/collection/item/endpoint/v1/endpoint.entities';
import {
  ExampleEntity,
  ExampleListItemEntity,
  ExampleVersionsItemEntity,
} from '../dist/meta/collection/item/example/v1/example.entities';
import { FolderListItemEntity } from '../dist/meta/collection/item/folder/v1/folder.entities';
import { ResponseGetResponseEntity } from '../dist/meta/collection/item/response/v1/response.entities';
import { MakeEndpointProps } from './resource';
import { createMethodKeyRecord, EndpointProps, makeEndpointFn, makeKey, makeListCollection } from './utils';

const listKeys: (keyof CollectionItemListRequest)[] = ['collectionId', 'parentFolderId'];

const itemSchema = new schema.Object({
  ...({} as CollectionItem),
  endpoint: EndpointListItemEntity,
  example: ExampleListItemEntity,
  folder: FolderListItemEntity,
});

const argsKey = (props: EndpointProps<typeof CollectionItemService.method.collectionItemList> | null) => {
  if (props === null) return {};
  const { input, transport } = props;
  return createMethodKeyRecord(transport, CollectionItemService.method.collectionItemList, input, listKeys);
};

const createCollectionFilter =
  ({ input, transport }: EndpointProps<typeof CollectionItemService.method.collectionItemList>) =>
  (collectionKey: Record<string, string>) => {
    const argsKey = createMethodKeyRecord(transport, CollectionItemService.method.collectionItemList, input, listKeys);
    const compare = Record.getEquivalence(Equivalence.string);
    return compare(argsKey, collectionKey);
  };

const items = new schema.Collection([itemSchema], { argsKey, createCollectionFilter });

export const list = ({ method, name }: MakeEndpointProps<typeof CollectionItemService.method.collectionItemList>) =>
  new Endpoint(makeEndpointFn(method), {
    key: makeKey(method, name),
    name,
    schema: { items },
  });

export const createFolder = ({ method, name }: MakeEndpointProps<typeof FolderService.method.folderCreate>) => {
  const endpointFn = async (props: EndpointProps<typeof FolderService.method.folderCreate>) => {
    const output = await makeEndpointFn(method)(props);
    const folder = Struct.omit({ ...props.input, ...output }, '$typeName');
    return create(CollectionItemSchema, { folder, kind: ItemKind.FOLDER });
  };

  return new Endpoint(endpointFn, {
    key: makeKey(method, name),
    name,
    schema: items.push,
    sideEffect: true,
  });
};

export const createEndpoint = ({
  method,
  name,
}: MakeEndpointProps<
  typeof EndpointService.method.endpointCreate | typeof EndpointService.method.endpointDuplicate
>) => {
  const endpointFn = async (
    props: EndpointProps<
      typeof EndpointService.method.endpointCreate | typeof EndpointService.method.endpointDuplicate
    >,
  ) => {
    const { endpointId, exampleId } = await makeEndpointFn(method)(props);
    return create(CollectionItemSchema, {
      endpoint: Struct.omit({ endpointId, method: 'GET', ...props.input }, '$typeName'),
      example: { exampleId },
      kind: ItemKind.ENDPOINT,
    });
  };

  return new Endpoint(endpointFn, {
    key: makeKey(method, name),
    name,
    schema: items.push,
    sideEffect: true,
  });
};

export const runExample = ({ method, name }: MakeEndpointProps<typeof ExampleService.method.exampleRun>) => {
  const endpointFn = async (props: EndpointProps<typeof ExampleService.method.exampleRun>) => {
    const output = await makeEndpointFn(method)(props);

    const example = create(ExampleSchema, {
      exampleId: props.input.exampleId!,
      lastResponseId: output.response!.responseId,
    });

    return { ...output, example };
  };

  // TODO: split version spec from example and simplify list schema
  const createCollectionFilter =
    ({ input, transport }: EndpointProps<typeof ExampleService.method.exampleRun>) =>
    (collectionKey: Record<string, string>) => {
      const argsKey = createMethodKeyRecord(transport, method, input, ['exampleId']);
      const compare = Record.getEquivalence(Equivalence.string);
      return compare(argsKey, collectionKey);
    };

  const versions = new schema.Collection([ExampleVersionsItemEntity], { createCollectionFilter });

  return new Endpoint(endpointFn, {
    key: makeKey(method, name),
    name,
    schema: {
      example: ExampleEntity,
      response: ResponseGetResponseEntity,
      version: versions.unshift,
    },
    sideEffect: true,
  });
};

export const move = ({ method, name }: MakeEndpointProps<typeof CollectionItemService.method.collectionItemMove>) => {
  const fromList = makeListCollection({
    inputPrimaryKeys: ['collectionId', 'parentFolderId'],
    itemSchema,
    method: CollectionItemService.method.collectionItemMove,
  });

  const toList = makeListCollection({
    argsKey: (props) => {
      if (props === null) return {};
      const { targetCollectionId, targetParentFolderId } = props.input;
      return createMethodKeyRecord(props.transport, method, {
        ...(targetCollectionId && { collectionId: targetCollectionId }),
        ...(targetParentFolderId && { parentFolderId: targetParentFolderId }),
      });
    },
    itemSchema,
    method: CollectionItemService.method.collectionItemMove,
  });

  const endpointFn = async (props: EndpointProps<typeof CollectionItemService.method.collectionItemMove>) => {
    await makeEndpointFn(method)(props);

    const snapshot = props.controller().snapshot(props.controller().getState());

    return Option.gen(function* () {
      const input = create(CollectionItemMoveRequestSchema, props.input);

      let from = yield* Option.fromNullable(snapshot.get(fromList, props));

      const isItem = (itemId: Uint8Array, kind: ItemKind) => (item: CollectionItem) => {
        if (item.kind !== kind) return false;
        if (kind === ItemKind.FOLDER && item.folder?.folderId.toString() === itemId.toString()) return true;
        if (kind === ItemKind.ENDPOINT && item.endpoint?.endpointId.toString() === itemId.toString()) return true;
        return false;
      };

      const fromIndex = yield* Array.findFirstIndex(from, isItem(input.itemId, input.kind));
      const item = Array.unsafeGet(from, fromIndex);
      from = Array.remove(from, fromIndex);

      const moveList =
        input.targetCollectionId !== undefined &&
        (input.collectionId.toString() !== input.targetCollectionId.toString() ||
          input.parentFolderId?.toString() !== input.targetParentFolderId?.toString());

      let to = Option.some(from);
      if (moveList) to = Option.fromNullable(snapshot.get(toList, props));

      let toIndex = 0;
      const { targetItemId, targetKind } = input;
      if (targetItemId && targetKind && Option.isSome(to)) {
        toIndex = yield* Array.findFirstIndex(to.value, isItem(targetItemId, targetKind));
        if (input.position === MovePosition.AFTER) toIndex += 1;
      }

      to = Option.flatMap(to, Array.insertAt(toIndex, item));

      const result: { from: CollectionItem[]; to?: CollectionItem[] } = {
        from: moveList ? from : (to as Option.Some<CollectionItem[]>).value,
      };

      if (moveList && Option.isSome(to)) result.to = to.value;

      return result;
    }).pipe(Option.getOrElse(() => ({})));
  };

  return new Endpoint(endpointFn, {
    key: makeKey(method, name),
    name,
    schema: { from: fromList, to: toList },
    sideEffect: true,
  });
};

export const moveExample = ({ method, name }: MakeEndpointProps<typeof ExampleService.method.exampleMove>) => {
  const list = makeListCollection({ inputPrimaryKeys: ['endpointId'], itemSchema: ExampleListItemEntity, method });

  const endpointFn = async (props: EndpointProps<typeof ExampleService.method.exampleMove>) => {
    await makeEndpointFn(method)(props);

    const snapshot = props.controller().snapshot(props.controller().getState());

    // TODO: implement a generic move helper
    return Option.gen(function* () {
      const items = yield* Option.fromNullable(snapshot.get(list, props));

      const { exampleId, position, targetExampleId } = create(ExampleMoveRequestSchema, props.input);

      const offset = yield* pipe(
        Match.value(position),
        Match.when(MovePosition.AFTER, () => 1),
        Match.when(MovePosition.BEFORE, () => 0),
        Match.option,
      );

      const { move = [], rest = [] } = Array.groupBy(items, (_) =>
        _.exampleId.toString() === exampleId.toString() ? 'move' : 'rest',
      );

      const index = yield* Array.findFirstIndex(rest, (_) => _.exampleId.toString() === targetExampleId.toString());

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
