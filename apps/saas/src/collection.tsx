import {
  createProtobufSafeUpdater,
  createQueryOptions,
  useQuery as useConnectQuery,
  useMutation,
  useTransport,
} from '@connectrpc/connect-query';
import { Schema } from '@effect/schema';
import { CollectionProps } from '@react-aria/collections';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import { Array, Effect, Match, pipe, Struct } from 'effect';
import { useRef, useState } from 'react';
import {
  UNSTABLE_TreeItem as AriaTreeItem,
  UNSTABLE_TreeItemContent as AriaTreeItemContent,
  TreeItemContentProps as AriaTreeItemContentProps,
  TreeItemProps as AriaTreeItemProps,
  Button,
  Collection,
  composeRenderProps,
  Dialog,
  FileTrigger,
  Form,
  Input,
  Label,
  ListBox,
  ListBoxItem,
  Popover,
  Select,
  SelectValue,
  Text,
  TextArea,
  TextField,
  UNSTABLE_Tree as Tree,
} from 'react-aria-components';
import { twMerge } from 'tailwind-merge';

import { ApiCall, CollectionMeta, Folder, Item } from '@the-dev-tools/protobuf/collection/v1/collection_pb';
import * as CollectionQuery from '@the-dev-tools/protobuf/collection/v1/collection-CollectionService_connectquery';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { composeRenderPropsTW } from '@the-dev-tools/ui/utils';
import { MixinProps, splitProps } from '@the-dev-tools/utils/mixin-props';

import { Runtime } from './runtime';

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceId');

export const CollectionsWidget = () => {
  const { workspaceId } = workspaceRoute.useParams();

  const transport = useTransport();
  const queryClient = useQueryClient();

  const createCollectionMutation = useMutation(CollectionQuery.createCollection);
  const collectionsQuery = useConnectQuery(CollectionQuery.listCollections, { workspaceId });

  const listQueryOptions = createQueryOptions(CollectionQuery.listCollections, { workspaceId }, { transport });

  if (!collectionsQuery.isSuccess) return null;
  const metaCollections = collectionsQuery.data.metaCollections;

  return (
    <>
      <h3 className='uppercase'>Collections</h3>
      <div className='flex justify-between gap-2'>
        <Button
          onPress={async () => {
            await createCollectionMutation.mutateAsync({ workspaceId, name: 'New collection' });
            await queryClient.invalidateQueries(listQueryOptions);
          }}
          className='flex-1 rounded bg-black text-white'
        >
          New
        </Button>
        <ImportPostman />
      </div>
      <Tree aria-label='Collections' items={metaCollections} className='flex flex-col gap-2'>
        {(_) => <CollectionWidget id={_.id} meta={_} />}
      </Tree>
    </>
  );
};

interface TreeItemProps<T extends object>
  extends Omit<AriaTreeItemProps, 'children'>,
    MixinProps<'content', Omit<AriaTreeItemContentProps, 'children'>>,
    MixinProps<'wrapper', Omit<React.ComponentProps<'div'>, 'children'>>,
    MixinProps<'child', Omit<CollectionProps<T>, 'children'>> {
  children?: AriaTreeItemContentProps['children'];
  childItem?: CollectionProps<T>['children'];
}

const TreeItem = <T extends object>({ children, className, childItem, ...mixProps }: TreeItemProps<T>) => {
  const props = splitProps(mixProps, 'content', 'wrapper', 'child');
  return (
    <AriaTreeItem {...props.rest} className={composeRenderPropsTW(className, tw`cursor-pointer select-none`)}>
      <AriaTreeItemContent {...props.content}>
        {composeRenderProps(children, (children, { hasChildRows, isExpanded, level }) => (
          <div
            {...props.wrapper}
            style={{ marginInlineStart: (level - 1).toString() + 'rem', ...props.wrapper.style }}
            className={twMerge(tw`flex gap-2`, props.wrapper.className)}
          >
            {hasChildRows && <Button slot='chevron'>{isExpanded ? '⏷' : '⏵'}</Button>}
            {!hasChildRows && level > 1 && <div />}
            {children}
          </div>
        ))}
      </AriaTreeItemContent>
      {!!childItem && <Collection {...props.child}>{childItem}</Collection>}
    </AriaTreeItem>
  );
};

interface CollectionWidgetProps {
  id: string;
  meta: CollectionMeta;
}

const CollectionWidget = ({ meta }: CollectionWidgetProps) => {
  const { workspaceId } = workspaceRoute.useParams();

  const transport = useTransport();
  const queryClient = useQueryClient();

  const deleteMutation = useMutation(CollectionQuery.deleteCollection);
  const updateMutation = useMutation(CollectionQuery.updateCollection);
  const createFolderMutation = useMutation(CollectionQuery.createFolder);

  const listQueryOptions = createQueryOptions(CollectionQuery.listCollections, { workspaceId }, { transport });

  const queryOptions = createQueryOptions(CollectionQuery.getCollection, { id: meta.id }, { transport });
  const query = useQuery({ ...queryOptions, enabled: true });

  const triggerRef = useRef(null);

  const [isRenaming, setIsRenaming] = useState(false);

  return (
    <TreeItem
      textValue={meta.name}
      childItems={query.data?.items ?? []}
      childItem={(_) => <ItemWidget id={_.data.value!.meta!.id} item={_} collectionId={meta.id} />}
    >
      <Text ref={triggerRef} className='flex-1 truncate'>{meta.name}</Text>

      <Button onPress={() => void setIsRenaming(true)}>Rename</Button>

      <Button
        onPress={async () => {
          await deleteMutation.mutateAsync({ id: meta.id });
          await queryClient.invalidateQueries(queryOptions);
          await queryClient.invalidateQueries(listQueryOptions);
        }}
      >
        Delete
      </Button>

      <Button
        onPress={async () => {
          await createFolderMutation.mutateAsync({ collectionId: meta.id, name: 'New folder' });
          await queryClient.invalidateQueries(queryOptions);
        }}
      >
        Create folder
      </Button>

      <Popover
        triggerRef={triggerRef}
        isOpen={isRenaming}
        onOpenChange={setIsRenaming}
        className='rounded border border-black bg-white p-2'
      >
        <Dialog aria-label='Rename collection'>
          <Form
            className='flex flex-1 gap-2'
            onSubmit={(event) =>
              Effect.gen(function* () {
                event.preventDefault();

                const { name } = yield* pipe(
                  new FormData(event.currentTarget),
                  Object.fromEntries,
                  Schema.decode(Schema.Struct({ name: Schema.String })),
                );

                yield* Effect.tryPromise(() => updateMutation.mutateAsync({ id: meta.id, name }));

                queryClient.setQueriesData(
                  queryOptions,
                  createProtobufSafeUpdater(
                    CollectionQuery.getCollection,
                    Struct.evolve({
                      name: () => name,
                    }),
                  ),
                );

                yield* Effect.tryPromise(() => queryClient.invalidateQueries(listQueryOptions));

                setIsRenaming(false);
              }).pipe(Runtime.runPromise)
            }
          >
            {/* eslint-disable-next-line jsx-a11y/no-autofocus */}
            <TextField name='name' defaultValue={meta.name} autoFocus className='contents'>
              <Label className='text-nowrap'>New name:</Label>
              <Input className='w-full bg-transparent' />
            </TextField>

            <Button type='submit'>Save</Button>
          </Form>
        </Dialog>
      </Popover>
    </TreeItem>
  );
};

interface ItemWidgetProps {
  id: string;
  item: Item;
  collectionId: string;
}

const ItemWidget = ({ item, collectionId }: ItemWidgetProps) =>
  pipe(
    item,
    Struct.get('data'),
    Match.value,
    Match.when({ case: 'folder' }, (_) => <FolderWidget folder={_.value} collectionId={collectionId} />),
    Match.when({ case: 'apiCall' }, (_) => <ApiCallWidget apiCall={_.value} collectionId={collectionId} />),
    Match.orElse(() => null),
  );

interface FolderWidgetProps {
  folder: Folder;
  collectionId: string;
}

const FolderWidget = ({ folder, collectionId }: FolderWidgetProps) => {
  const transport = useTransport();
  const queryClient = useQueryClient();

  const deleteMutation = useMutation(CollectionQuery.deleteFolder);
  const updateMutation = useMutation(CollectionQuery.updateFolder);

  const queryOptions = createQueryOptions(CollectionQuery.getCollection, { id: collectionId }, { transport });

  const triggerRef = useRef(null);

  const [isRenaming, setIsRenaming] = useState(false);

  return (
    <TreeItem
      textValue={folder.meta!.name}
      childItems={folder.items}
      childItem={(_) => <ItemWidget id={_.data.value!.meta!.id} item={_} collectionId={collectionId} />}
    >
      <div>FOLDER</div>
      <Text ref={triggerRef} className='flex-1 truncate'>
        {folder.meta!.name}
      </Text>
      <Button onPress={() => void setIsRenaming(true)}>Rename</Button>
      <Button
        onPress={async () => {
          await deleteMutation.mutateAsync({ collectionId, id: folder.meta!.id });
          await queryClient.invalidateQueries(queryOptions);
        }}
      >
        Delete
      </Button>

      <Popover
        triggerRef={triggerRef}
        isOpen={isRenaming}
        onOpenChange={setIsRenaming}
        className='rounded border border-black bg-white p-2'
      >
        <Dialog aria-label='Rename folder'>
          <Form
            className='flex flex-1 gap-2'
            onSubmit={(event) =>
              Effect.gen(function* () {
                event.preventDefault();

                const { name } = yield* pipe(
                  new FormData(event.currentTarget),
                  Object.fromEntries,
                  Schema.decode(Schema.Struct({ name: Schema.String })),
                );

                yield* Effect.tryPromise(() =>
                  updateMutation.mutateAsync({
                    folder: {
                      ...Struct.evolve(folder, {
                        meta: Struct.evolve({ name: () => name }),
                      }),
                      collectionId,
                    },
                  }),
                );

                yield* Effect.tryPromise(() => queryClient.invalidateQueries(queryOptions));

                setIsRenaming(false);
              }).pipe(Runtime.runPromise)
            }
          >
            {/* eslint-disable-next-line jsx-a11y/no-autofocus */}
            <TextField name='name' defaultValue={folder.meta!.name} autoFocus className='contents'>
              <Label className='text-nowrap'>New name:</Label>
              <Input className='w-full bg-transparent' />
            </TextField>

            <Button type='submit'>Save</Button>
          </Form>
        </Dialog>
      </Popover>
    </TreeItem>
  );
};

interface ApiCallWidgetProps {
  apiCall: ApiCall;
  collectionId: string;
}

const ApiCallWidget = ({ apiCall, collectionId }: ApiCallWidgetProps) => {
  const transport = useTransport();
  const queryClient = useQueryClient();

  const { workspaceId } = workspaceRoute.useParams();

  const runNodeMutation = useMutation(CollectionQuery.runApiCall);
  const deleteMutation = useMutation(CollectionQuery.deleteApiCall);

  const queryOptions = createQueryOptions(CollectionQuery.getCollection, { id: collectionId }, { transport });

  return (
    <TreeItem
      textValue={apiCall.meta!.name}
      href={{ to: '/workspace/$workspaceId/api-call/$apiCallId', params: { workspaceId, apiCallId: apiCall.meta!.id } }}
    >
      <div>{apiCall.data!.method}</div>
      <Text className='flex-1 truncate'>{apiCall.meta!.name}</Text>
      {runNodeMutation.isSuccess && <div>Duration: {runNodeMutation.data.result!.duration.toString()} ms</div>}
      <Button
        onPress={async () => {
          await deleteMutation.mutateAsync({ collectionId, id: apiCall.meta!.id });
          await queryClient.invalidateQueries(queryOptions);
        }}
      >
        Delete
      </Button>
      <Button onPress={() => void runNodeMutation.mutate({ id: apiCall.meta!.id })}>Run</Button>
    </TreeItem>
  );
};

const ImportPostman = () => {
  const { workspaceId } = workspaceRoute.useParams();

  const transport = useTransport();
  const queryClient = useQueryClient();

  const createMutation = useMutation(CollectionQuery.importPostman);

  const listQueryOptions = createQueryOptions(CollectionQuery.listCollections, { workspaceId }, { transport });

  return (
    <FileTrigger
      onSelect={async (_) => {
        const file = _?.item(0);
        if (!file) return;
        await createMutation.mutateAsync({
          workspaceId,
          name: file.name,
          data: new Uint8Array(await file.arrayBuffer()),
        });
        await queryClient.invalidateQueries(listQueryOptions);
      }}
    >
      <Button className='flex-1 rounded bg-black text-white'>Import</Button>
    </FileTrigger>
  );
};

const apiCallRoute = getRouteApi('/_authorized/workspace/$workspaceId/api-call/$apiCallId');

class ApiCallForm extends Schema.Class<ApiCallForm>('ApiCallForm')({
  name: Schema.String,
  method: Schema.Literal('GET', 'HEAD', 'POST', 'PUT', 'DELETE', 'CONNECT', 'OPTION', 'TRACE', 'PATCH'),
  url: Schema.String,
  body: Schema.String,
}) {}

export const ApiCallPage = () => {
  const { apiCallId } = apiCallRoute.useParams();

  const updateMutation = useMutation(CollectionQuery.updateApiCall);

  const query = useConnectQuery(CollectionQuery.getApiCall, { id: apiCallId });
  if (!query.isSuccess) return null;
  const { data } = query;

  const body = new TextDecoder().decode(data.apiCall!.data!.body);

  return (
    <>
      <h2 className='truncate text-center text-2xl font-extrabold'>{data.apiCall!.meta!.name}</h2>

      <div className='my-2 h-px bg-black' />

      <Form
        onSubmit={(event) =>
          Effect.gen(function* () {
            event.preventDefault();

            const { name, method, url, body } = yield* pipe(
              new FormData(event.currentTarget),
              Object.fromEntries,
              Schema.decode(ApiCallForm),
            );

            const newApiCall = Struct.evolve(data.apiCall!, {
              meta: (_) => Struct.evolve(_!, { name: () => name }),
              data: (_) =>
                Struct.evolve(_!, {
                  method: () => method,
                  url: () => url,
                  body: () => new TextEncoder().encode(body),
                }),
            });

            yield* Effect.tryPromise(() => updateMutation.mutateAsync({ apiCall: newApiCall }));
          }).pipe(Runtime.runPromise)
        }
      >
        <TextField name='name' defaultValue={data.apiCall!.meta!.name} className='flex gap-2'>
          <Label>Name:</Label>
          <Input className='flex-1' />
        </TextField>

        <Select name='method' defaultSelectedKey={data.apiCall!.data!.method} className='flex gap-2'>
          <Label>Method:</Label>
          <Button>
            <SelectValue />
          </Button>
          <Popover className='bg-white'>
            <ListBox>
              {Array.map(ApiCallForm.fields.method.literals, (_) => (
                <ListBoxItem key={_} id={_} className='cursor-pointer'>
                  {_}
                </ListBoxItem>
              ))}
            </ListBox>
          </Popover>
        </Select>

        <TextField name='url' defaultValue={data.apiCall!.data!.url} className='flex gap-2'>
          <Label>URL:</Label>
          <Input className='flex-1' />
        </TextField>

        <TextField name='body' defaultValue={body} className='flex gap-2'>
          <Label>Body:</Label>
          <TextArea className='flex-1' />
        </TextField>

        <Button type='submit'>Save</Button>
      </Form>
    </>
  );
};
