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
import { getRouteApi, useMatch } from '@tanstack/react-router';
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
  Menu,
  MenuItem,
  MenuTrigger,
  Popover,
  Select,
  SelectValue,
  Text,
  TextField,
  UNSTABLE_Tree as Tree,
} from 'react-aria-components';
import { LuChevronRight, LuFolder, LuMoreHorizontal } from 'react-icons/lu';
import { twJoin, twMerge } from 'tailwind-merge';

import { CollectionMeta } from '@the-dev-tools/protobuf/collection/v1/collection_pb';
import {
  createCollection,
  deleteCollection,
  getCollection,
  importPostman,
  listCollections,
  updateCollection,
} from '@the-dev-tools/protobuf/collection/v1/collection-CollectionService_connectquery';
import { ApiCall } from '@the-dev-tools/protobuf/itemapi/v1/itemapi_pb';
import {
  deleteApiCall,
  getApiCall,
  updateApiCall,
} from '@the-dev-tools/protobuf/itemapi/v1/itemapi-ItemApiService_connectquery';
import { Folder, Item } from '@the-dev-tools/protobuf/itemfolder/v1/itemfolder_pb';
import {
  createFolder,
  deleteFolder,
  updateFolder,
} from '@the-dev-tools/protobuf/itemfolder/v1/itemfolder-ItemFolderService_connectquery';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { composeRenderPropsTW } from '@the-dev-tools/ui/utils';
import { MixinProps, splitProps } from '@the-dev-tools/utils/mixin-props';

import { Runtime } from './runtime';

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceId');

export const CollectionsWidget = () => {
  const { workspaceId } = workspaceRoute.useParams();

  const transport = useTransport();
  const queryClient = useQueryClient();

  const createCollectionMutation = useMutation(createCollection);
  const collectionsQuery = useConnectQuery(listCollections, { workspaceId });

  const listQueryOptions = createQueryOptions(listCollections, { workspaceId }, { transport });

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
    <AriaTreeItem
      {...props.rest}
      className={composeRenderPropsTW(className, tw`group cursor-pointer select-none outline-none`)}
    >
      <AriaTreeItemContent {...props.content}>
        {composeRenderProps(children, (children, { hasChildRows, isExpanded, level }) => (
          <div
            {...props.wrapper}
            style={{ marginInlineStart: (level - 1).toString() + 'rem', ...props.wrapper.style }}
            className={twMerge(
              tw`flex items-center gap-2 p-1 outline outline-0 group-rac-focus-visible:outline-2`,
              props.wrapper.className,
            )}
          >
            {hasChildRows && (
              <Button slot='chevron'>
                <LuChevronRight
                  className={twJoin(tw`transition-transform`, !isExpanded ? tw`rotate-0` : tw`rotate-90`)}
                />
              </Button>
            )}
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

  const deleteMutation = useMutation(deleteCollection);
  const updateMutation = useMutation(updateCollection);
  const createFolderMutation = useMutation(createFolder);

  const listQueryOptions = createQueryOptions(listCollections, { workspaceId }, { transport });

  const queryOptions = createQueryOptions(getCollection, { id: meta.id }, { transport });
  const query = useQuery({ ...queryOptions, enabled: true });

  const triggerRef = useRef(null);

  const [isRenaming, setIsRenaming] = useState(false);

  return (
    <TreeItem
      textValue={meta.name}
      childItems={query.data?.items ?? []}
      childItem={(_) => <ItemWidget id={_.data.value!.meta!.id} item={_} collectionId={meta.id} />}
    >
      <Text ref={triggerRef} className='flex-1 truncate'>
        {meta.name}
      </Text>

      <MenuTrigger>
        <Button>
          <LuMoreHorizontal />
        </Button>

        <Popover>
          <Menu className='flex flex-col gap-2 rounded border-2 border-black bg-white p-2'>
            <MenuItem className='cursor-pointer select-none' onAction={() => void setIsRenaming(true)}>
              Rename
            </MenuItem>

            <MenuItem
              className='cursor-pointer select-none'
              onAction={async () => {
                await deleteMutation.mutateAsync({ id: meta.id });
                await queryClient.invalidateQueries(listQueryOptions);
                await queryClient.invalidateQueries(queryOptions);
              }}
            >
              Delete
            </MenuItem>

            <MenuItem
              className='cursor-pointer select-none'
              onAction={async () => {
                await createFolderMutation.mutateAsync({ collectionId: meta.id, name: 'New folder' });
                await queryClient.invalidateQueries(queryOptions);
              }}
            >
              Create folder
            </MenuItem>
          </Menu>
        </Popover>
      </MenuTrigger>

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
                    getCollection,
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

  const deleteMutation = useMutation(deleteFolder);
  const updateMutation = useMutation(updateFolder);

  const queryOptions = createQueryOptions(getCollection, { id: collectionId }, { transport });

  const triggerRef = useRef(null);

  const [isRenaming, setIsRenaming] = useState(false);

  return (
    <TreeItem
      textValue={folder.meta!.name}
      childItems={folder.items}
      childItem={(_) => <ItemWidget id={_.data.value!.meta!.id} item={_} collectionId={collectionId} />}
    >
      <LuFolder />

      <Text ref={triggerRef} className='flex-1 truncate'>
        {folder.meta!.name}
      </Text>

      <MenuTrigger>
        <Button>
          <LuMoreHorizontal />
        </Button>

        <Popover>
          <Menu className='flex flex-col gap-2 rounded border-2 border-black bg-white p-2'>
            <MenuItem className='cursor-pointer select-none' onAction={() => void setIsRenaming(true)}>
              Rename
            </MenuItem>

            <MenuItem
              className='cursor-pointer select-none'
              onAction={async () => {
                await deleteMutation.mutateAsync({ collectionId, id: folder.meta!.id });
                await queryClient.invalidateQueries(queryOptions);
              }}
            >
              Delete
            </MenuItem>
          </Menu>
        </Popover>
      </MenuTrigger>

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

  const match = useMatch({ strict: false });

  const { workspaceId } = workspaceRoute.useParams();

  const deleteMutation = useMutation(deleteApiCall);

  const queryOptions = createQueryOptions(getCollection, { id: collectionId }, { transport });

  return (
    <TreeItem
      textValue={apiCall.meta!.name}
      href={{ to: '/workspace/$workspaceId/api-call/$apiCallId', params: { workspaceId, apiCallId: apiCall.meta!.id } }}
      wrapperClassName={twJoin(match.params.apiCallId === apiCall.meta!.id && tw`bg-black text-white`)}
    >
      <div />

      <div className='text-sm font-bold'>{apiCall.method}</div>

      <Text className='flex-1 truncate'>{apiCall.meta!.name}</Text>

      <MenuTrigger>
        <Button>
          <LuMoreHorizontal />
        </Button>

        <Popover>
          <Menu className='flex flex-col gap-2 rounded border-2 border-black bg-white p-2'>
            <MenuItem
              className='cursor-pointer select-none'
              onAction={async () => {
                await deleteMutation.mutateAsync({ collectionId, id: apiCall.meta!.id });
                await queryClient.invalidateQueries(queryOptions);
              }}
            >
              Delete
            </MenuItem>
          </Menu>
        </Popover>
      </MenuTrigger>
    </TreeItem>
  );
};

const ImportPostman = () => {
  const { workspaceId } = workspaceRoute.useParams();

  const transport = useTransport();
  const queryClient = useQueryClient();

  const createMutation = useMutation(importPostman);

  const listQueryOptions = createQueryOptions(listCollections, { workspaceId }, { transport });

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

  const updateMutation = useMutation(updateApiCall);

  const query = useConnectQuery(getApiCall, { id: apiCallId });
  if (!query.isSuccess) return null;
  const { data } = query;

  return (
    <>
      <h2 className='truncate text-center text-2xl font-extrabold'>{data.apiCall!.meta!.name}</h2>

      <div className='my-2 h-px bg-black' />

      <Form
        onSubmit={(event) =>
          Effect.gen(function* () {
            event.preventDefault();

            const { name, method, url } = yield* pipe(
              new FormData(event.currentTarget),
              Object.fromEntries,
              Schema.decode(ApiCallForm),
            );

            const newApiCall = Struct.evolve(data.apiCall!, {
              meta: (_) => Struct.evolve(_!, { name: () => name }),
              method: () => method,
              url: () => url,
            });

            yield* Effect.tryPromise(() => updateMutation.mutateAsync({ apiCall: newApiCall }));
          }).pipe(Runtime.runPromise)
        }
      >
        <TextField name='name' defaultValue={data.apiCall!.meta!.name} className='flex gap-2'>
          <Label>Name:</Label>
          <Input className='flex-1' />
        </TextField>

        <Select name='method' defaultSelectedKey={data.apiCall!.method} className='flex gap-2'>
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

        <TextField name='url' defaultValue={data.apiCall!.url} className='flex gap-2'>
          <Label>URL:</Label>
          <Input className='flex-1' />
        </TextField>

        <Button type='submit'>Save</Button>
      </Form>
    </>
  );
};
