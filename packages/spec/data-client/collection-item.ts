import { create, MessageInitShape } from '@bufbuild/protobuf';
import { Transport } from '@connectrpc/connect';
import { Endpoint, schema } from '@data-client/endpoint';
import { Equivalence, Record, Struct } from 'effect';

import {
  EndpointCreateRequestSchema,
  EndpointCreateResponseSchema,
  EndpointDuplicateRequestSchema,
  EndpointDuplicateResponseSchema,
} from '../dist/buf/typescript/collection/item/endpoint/v1/endpoint_pb';
import {
  ExampleRunRequestSchema,
  ExampleRunResponseSchema,
} from '../dist/buf/typescript/collection/item/example/v1/example_pb';
import {
  FolderCreateRequestSchema,
  FolderCreateResponseSchema,
} from '../dist/buf/typescript/collection/item/folder/v1/folder_pb';
import {
  CollectionItem,
  CollectionItemListRequest,
  CollectionItemListRequestSchema,
  CollectionItemListResponseSchema,
  CollectionItemSchema,
  CollectionItemService,
  ItemKind,
} from '../dist/buf/typescript/collection/item/v1/item_pb';
import { EndpointListItemEntity } from '../dist/meta/collection/item/endpoint/v1/endpoint.entities';
import {
  ExampleEntity,
  ExampleListItemEntity,
  ExampleVersionsItemEntity,
} from '../dist/meta/collection/item/example/v1/example.entities';
import { FolderListItemEntity } from '../dist/meta/collection/item/folder/v1/folder.entities';
import { ResponseGetResponseEntity } from '../dist/meta/collection/item/response/v1/response.entities';
import { EndpointProps } from './resource';
import { createMethodKey, createMethodKeyRecord, fetchMethod } from './utils';

const listKeys: (keyof CollectionItemListRequest)[] = ['collectionId', 'parentFolderId'];

const itemSchema = new schema.Object({
  ...({} as CollectionItem),
  endpoint: EndpointListItemEntity,
  example: ExampleListItemEntity,
  folder: FolderListItemEntity,
});

type args = [Transport, MessageInitShape<typeof CollectionItemListRequestSchema>];

const argsKey = (...args: [null] | args) => {
  if (args[0] === null) return {};
  const [transport, input] = args;
  return createMethodKeyRecord(transport, CollectionItemService.method.collectionItemList, input, listKeys);
};

const createCollectionFilter =
  (...[transport, input]: args) =>
  (collectionKey: Record<string, string>) => {
    const argsKey = createMethodKeyRecord(transport, CollectionItemService.method.collectionItemList, input, listKeys);
    const compare = Record.getEquivalence(Equivalence.string);
    return compare(argsKey, collectionKey);
  };

const items = new schema.Collection([itemSchema], { argsKey, createCollectionFilter });

export const list = ({
  method,
  name,
}: EndpointProps<typeof CollectionItemListRequestSchema, typeof CollectionItemListResponseSchema>) => {
  const fetchFunction = (transport: Transport, input: MessageInitShape<typeof CollectionItemListRequestSchema>) =>
    fetchMethod(transport, method, input);

  const key = (...[transport, input]: Parameters<typeof fetchFunction>) =>
    name + ':' + createMethodKey(transport, method, input);

  return new Endpoint(fetchFunction, { key, name, schema: { items } });
};

export const createFolder = ({
  method,
  name,
}: EndpointProps<typeof FolderCreateRequestSchema, typeof FolderCreateResponseSchema>) => {
  const fetchFunction = async (transport: Transport, input: MessageInitShape<typeof FolderCreateRequestSchema>) => {
    const output = await fetchMethod(transport, method, input);
    const folder = Struct.omit({ ...input, ...output }, '$typeName');
    return create(CollectionItemSchema, { folder, kind: ItemKind.FOLDER });
  };

  const key = (...[transport, input]: Parameters<typeof fetchFunction>) =>
    name + ':' + createMethodKey(transport, method, input);

  return new Endpoint(fetchFunction, { key, name, schema: items.push, sideEffect: true });
};

export const createEndpoint = ({
  method,
  name,
}: EndpointProps<
  typeof EndpointCreateRequestSchema | typeof EndpointDuplicateRequestSchema,
  typeof EndpointCreateResponseSchema | typeof EndpointDuplicateResponseSchema
>) => {
  const fetchFunction = async (transport: Transport, input: MessageInitShape<typeof EndpointCreateRequestSchema>) => {
    const output = await fetchMethod(transport, method, input);
    const endpoint = Struct.omit({ ...input, ...output }, '$typeName');
    return create(CollectionItemSchema, { endpoint, kind: ItemKind.ENDPOINT });
  };

  const key = (...[transport, input]: Parameters<typeof fetchFunction>) =>
    name + ':' + createMethodKey(transport, method, input);

  return new Endpoint(fetchFunction, { key, name, schema: items.push, sideEffect: true });
};

export const runExample = ({
  method,
  name,
}: EndpointProps<typeof ExampleRunRequestSchema, typeof ExampleRunResponseSchema>) => {
  const fetchFunction = async (transport: Transport, input: MessageInitShape<typeof ExampleRunRequestSchema>) => {
    const output = await fetchMethod(transport, method, input);

    const example = {
      exampleId: input.exampleId,
      lastResponseId: output.response?.responseId,
    };

    return { ...output, example };
  };

  const key = (...[transport, input]: Parameters<typeof fetchFunction>) =>
    name + ':' + createMethodKey(transport, method, input);

  // TODO: split version spec from example and simplify list schema
  const createCollectionFilter =
    (...[transport, input]: Parameters<typeof fetchFunction>) =>
    (collectionKey: Record<string, string>) => {
      const argsKey = createMethodKeyRecord(transport, method, input, ['exampleId']);
      const compare = Record.getEquivalence(Equivalence.string);
      return compare(argsKey, collectionKey);
    };

  const versions = new schema.Collection([ExampleVersionsItemEntity], { createCollectionFilter });

  return new Endpoint(fetchFunction, {
    key,
    name,
    schema: {
      example: ExampleEntity,
      response: ResponseGetResponseEntity,
      version: versions.unshift,
    },
    sideEffect: true,
  });
};
