import { create } from '@bufbuild/protobuf';
import { Endpoint, schema } from '@data-client/endpoint';
import { Equivalence, Record, Struct } from 'effect';

import { EndpointService } from '../dist/buf/typescript/collection/item/endpoint/v1/endpoint_pb';
import { ExampleService } from '../dist/buf/typescript/collection/item/example/v1/example_pb';
import { FolderService } from '../dist/buf/typescript/collection/item/folder/v1/folder_pb';
import {
  CollectionItem,
  CollectionItemListRequest,
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
import { MakeEndpointProps } from './resource';
import { createMethodKeyRecord, EndpointProps, makeEndpointFn, makeKey } from './utils';

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

    const example = {
      exampleId: props.input.exampleId,
      lastResponseId: output.response?.responseId,
    };

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
