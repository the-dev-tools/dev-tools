import {
  createQueryOptions,
  useMutation as useConnectMutation,
  useQuery as useConnectQuery,
  useTransport,
} from '@connectrpc/connect-query';
import { useQueryClient } from '@tanstack/react-query';
import { getRouteApi, useMatch } from '@tanstack/react-router';
import { Effect, Match, pipe, Runtime, Schema } from 'effect';
import { Ulid } from 'id128';
import { useMemo, useRef, useState } from 'react';
import { Form, MenuTrigger, Text, UNSTABLE_Tree as Tree } from 'react-aria-components';
import { FiFolder, FiMoreHorizontal, FiRotateCw } from 'react-icons/fi';
import { MdLightbulbOutline } from 'react-icons/md';

import { useSpecMutation } from '@the-dev-tools/api/query';
import { collectionDeleteSpec, collectionUpdateSpec } from '@the-dev-tools/api/spec/collection';
import { endpointCreateSpec, endpointDeleteSpec } from '@the-dev-tools/api/spec/collection/item/endpoint';
import { exampleCreateSpec, exampleDeleteSpec } from '@the-dev-tools/api/spec/collection/item/example';
import { folderCreateSpec, folderDeleteSpec, folderUpdateSpec } from '@the-dev-tools/api/spec/collection/item/folder';
import { Endpoint, EndpointListItem } from '@the-dev-tools/spec/collection/item/endpoint/v1/endpoint_pb';
import { endpointDuplicate } from '@the-dev-tools/spec/collection/item/endpoint/v1/endpoint-EndpointService_connectquery';
import { ExampleListItem } from '@the-dev-tools/spec/collection/item/example/v1/example_pb';
import {
  exampleDuplicate,
  exampleList,
} from '@the-dev-tools/spec/collection/item/example/v1/example-ExampleService_connectquery';
import { Folder, FolderListItem } from '@the-dev-tools/spec/collection/item/folder/v1/folder_pb';
import { CollectionItem, ItemKind } from '@the-dev-tools/spec/collection/item/v1/item_pb';
import { collectionItemList } from '@the-dev-tools/spec/collection/item/v1/item-CollectionItemService_connectquery';
import { Collection, CollectionListItem } from '@the-dev-tools/spec/collection/v1/collection_pb';
import { collectionList } from '@the-dev-tools/spec/collection/v1/collection-CollectionService_connectquery';
import { Button } from '@the-dev-tools/ui/button';
import { FolderOpenedIcon } from '@the-dev-tools/ui/icons';
import { Menu, MenuItem } from '@the-dev-tools/ui/menu';
import { MethodBadge } from '@the-dev-tools/ui/method-badge';
import { Popover } from '@the-dev-tools/ui/popover';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField } from '@the-dev-tools/ui/text-field';
import { TreeItem } from '@the-dev-tools/ui/tree';

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceIdCan');

const useInvalidateCollectionListQuery = () => {
  const { workspaceId } = workspaceRoute.useLoaderData();
  const queryClient = useQueryClient();
  const transport = useTransport();
  const collectionListQueryOptions = createQueryOptions(collectionList, { workspaceId }, { transport });
  return () => queryClient.invalidateQueries(collectionListQueryOptions);
};

export const CollectionListTree = () => {
  const { workspaceId } = workspaceRoute.useLoaderData();

  const collectionListQuery = useConnectQuery(collectionList, { workspaceId });

  if (!collectionListQuery.isSuccess) return null;
  const collections = collectionListQuery.data.items;

  return (
    <Tree aria-label='Collections' items={collections}>
      {(_) => {
        const collectionIdCan = Ulid.construct(_.collectionId).toCanonical();
        return <CollectionTree id={collectionIdCan} collection={_} />;
      }}
    </Tree>
  );
};

interface CollectionTreeProps {
  id: string;
  collection: CollectionListItem;
}

const CollectionTree = ({ collection }: CollectionTreeProps) => {
  const { workspaceId } = workspaceRoute.useLoaderData();
  const { runtime } = workspaceRoute.useRouteContext();

  const { collectionId } = collection;
  const [enabled, setEnabled] = useState(false);

  const collectionItemListQuery = useConnectQuery(collectionItemList, { collectionId }, { enabled });
  const collectionDeleteMutation = useSpecMutation(collectionDeleteSpec);
  const collectionUpdateMutation = useSpecMutation(collectionUpdateSpec);

  const folderCreateMutation = useSpecMutation(folderCreateSpec);
  const endpointCreateMutation = useSpecMutation(endpointCreateSpec);

  const triggerRef = useRef(null);

  const [isRenaming, setIsRenaming] = useState(false);

  const childItems = useMemo(
    () => (collectionItemListQuery.data?.items ?? []).filter((_) => _.kind !== ItemKind.UNSPECIFIED),
    [collectionItemListQuery.data?.items],
  );

  return (
    <TreeItem
      textValue={collection.name}
      childItems={childItems}
      childItem={mapCollectionItemTree(collectionId)}
      expandButtonIsForced={!enabled}
      expandButtonOnPress={() => void setEnabled(true)}
    >
      {collectionItemListQuery.isLoading && (
        <Button variant='ghost' isDisabled className={tw`p-1`}>
          <FiRotateCw className={tw`size-3 animate-spin text-slate-500`} />
        </Button>
      )}

      <Text ref={triggerRef} className='flex-1 truncate'>
        {collection.name}
      </Text>

      <MenuTrigger>
        <Button variant='ghost' className={tw`p-0.5`}>
          <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
        </Button>

        <Menu>
          <MenuItem onAction={() => void setIsRenaming(true)}>Rename</MenuItem>

          <MenuItem onAction={() => void endpointCreateMutation.mutate({ collectionId, name: 'New API call' })}>
            Add Request
          </MenuItem>

          <MenuItem onAction={() => void folderCreateMutation.mutate({ collectionId, name: 'New folder' })}>
            Add Folder
          </MenuItem>

          <MenuItem
            variant='danger'
            onAction={() => void collectionDeleteMutation.mutate({ workspaceId, collectionId })}
          >
            Delete
          </MenuItem>
        </Menu>
      </MenuTrigger>

      <Popover
        triggerRef={triggerRef}
        isOpen={isRenaming}
        onOpenChange={setIsRenaming}
        dialogAria-label='Rename collection'
      >
        <Form
          className='flex flex-1 items-center gap-2'
          onSubmit={(event) =>
            Effect.gen(function* () {
              event.preventDefault();

              const { name } = yield* pipe(
                new FormData(event.currentTarget),
                Object.fromEntries,
                Schema.decode(Schema.Struct({ name: Schema.String })),
              );

              collectionUpdateMutation.mutate({ workspaceId, collectionId, name });

              setIsRenaming(false);
            }).pipe(Runtime.runPromise(runtime))
          }
        >
          <TextField
            name='name'
            defaultValue={collection.name}
            // eslint-disable-next-line jsx-a11y/no-autofocus
            autoFocus
            label='New name:'
            className={tw`contents`}
            labelClassName={tw`text-nowrap`}
            inputClassName={tw`w-full bg-transparent`}
          />

          <Button type='submit'>Save</Button>
        </Form>
      </Popover>
    </TreeItem>
  );
};

const mapCollectionItemTree =
  (collectionId: Collection['collectionId'], parentFolderId?: Folder['folderId']) => (item: CollectionItem) =>
    pipe(
      Match.value(item),
      Match.when({ kind: ItemKind.FOLDER }, (_) => {
        const folderIdCan = Ulid.construct(_.folder!.folderId).toCanonical();
        return (
          <FolderTree id={folderIdCan} collectionId={collectionId} parentFolderId={parentFolderId} folder={_.folder!} />
        );
      }),
      Match.when({ kind: ItemKind.ENDPOINT }, (_) => {
        const endpointIdCan = Ulid.construct(_.endpoint!.endpointId).toCanonical();
        return (
          <EndpointTree
            id={endpointIdCan}
            collectionId={collectionId}
            parentFolderId={parentFolderId}
            endpoint={_.endpoint!}
            example={_.example!}
          />
        );
      }),
      Match.orElse(() => null),
    );

interface FolderTreeProps {
  id: string;
  collectionId: Collection['collectionId'];
  parentFolderId: Folder['folderId'] | undefined;
  folder: FolderListItem;
}

const FolderTree = ({ collectionId, parentFolderId, folder }: FolderTreeProps) => {
  const { runtime } = workspaceRoute.useRouteContext();

  const { folderId } = folder;
  const [enabled, setEnabled] = useState(false);

  const collectionItemListQuery = useConnectQuery(collectionItemList, { collectionId, folderId }, { enabled });

  const childItems = useMemo(
    () => (collectionItemListQuery.data?.items ?? []).filter((_) => _.kind !== ItemKind.UNSPECIFIED),
    [collectionItemListQuery.data?.items],
  );

  const folderCreateMutation = useSpecMutation(folderCreateSpec);
  const folderUpdateMutation = useSpecMutation(folderUpdateSpec);
  const folderDeleteMutation = useSpecMutation(folderDeleteSpec);

  const endpointCreateMutation = useSpecMutation(endpointCreateSpec);

  const triggerRef = useRef(null);

  const [isRenaming, setIsRenaming] = useState(false);

  return (
    <TreeItem
      textValue={folder.name}
      childItems={childItems}
      childItem={mapCollectionItemTree(collectionId, folderId)}
      expandButtonIsForced={!enabled}
      expandButtonOnPress={() => void setEnabled(true)}
    >
      {({ isExpanded }) => (
        <>
          {collectionItemListQuery.isLoading && (
            <Button variant='ghost' isDisabled className={tw`p-1`}>
              <FiRotateCw className={tw`size-3 animate-spin text-slate-500`} />
            </Button>
          )}

          {isExpanded ? (
            <FolderOpenedIcon className={tw`size-4 text-slate-500`} />
          ) : (
            <FiFolder className={tw`size-4 text-slate-500`} />
          )}

          <Text ref={triggerRef} className='flex-1 truncate'>
            {folder.name}
          </Text>

          <MenuTrigger>
            <Button variant='ghost' className={tw`p-0.5`}>
              <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
            </Button>

            <Menu>
              <MenuItem onAction={() => void setIsRenaming(true)}>Rename</MenuItem>

              <MenuItem
                onAction={() =>
                  void endpointCreateMutation.mutate({
                    collectionId,
                    parentFolderId: folderId,
                    name: 'New API call',
                  })
                }
              >
                Add Request
              </MenuItem>

              <MenuItem
                onAction={() =>
                  void folderCreateMutation.mutate({
                    collectionId,
                    parentFolderId: folderId,
                    name: 'New folder',
                  })
                }
              >
                Add Folder
              </MenuItem>

              <MenuItem
                variant='danger'
                onAction={() =>
                  void folderDeleteMutation.mutate({ collectionId, folderId, parentFolderId: parentFolderId! })
                }
              >
                Delete
              </MenuItem>
            </Menu>
          </MenuTrigger>

          <Popover
            triggerRef={triggerRef}
            isOpen={isRenaming}
            onOpenChange={setIsRenaming}
            dialogAria-label='Rename folder'
          >
            <Form
              className='flex flex-1 items-center gap-2'
              onSubmit={(event) =>
                Effect.gen(function* () {
                  event.preventDefault();

                  const { name } = yield* pipe(
                    new FormData(event.currentTarget),
                    Object.fromEntries,
                    Schema.decode(Schema.Struct({ name: Schema.String })),
                  );

                  folderUpdateMutation.mutate({
                    collectionId,
                    folderId,
                    name,
                    parentFolderId: parentFolderId!,
                  });

                  setIsRenaming(false);
                }).pipe(Runtime.runPromise(runtime))
              }
            >
              <TextField
                name='name'
                defaultValue={folder.name}
                // eslint-disable-next-line jsx-a11y/no-autofocus
                autoFocus
                label='New name:'
                className={tw`contents`}
                labelClassName={tw`text-nowrap`}
                inputClassName={tw`w-full bg-transparent`}
              />

              <Button type='submit'>Save</Button>
            </Form>
          </Popover>
        </>
      )}
    </TreeItem>
  );
};

interface EndpointTreeProps {
  id: string;
  collectionId: Collection['collectionId'];
  parentFolderId: Folder['folderId'] | undefined;
  endpoint: EndpointListItem;
  example: ExampleListItem;
}

const EndpointTree = ({ id: endpointIdCan, collectionId, parentFolderId, endpoint, example }: EndpointTreeProps) => {
  const match = useMatch({ strict: false });

  const exampleIdCan = Ulid.construct(example.exampleId).toCanonical();
  const { endpointId, method } = endpoint;

  const [enabled, setEnabled] = useState(false);

  const exampleListQuery = useConnectQuery(exampleList, { endpointId }, { enabled });

  const invalidateCollectionListQuery = useInvalidateCollectionListQuery();

  const exampleCreateMutation = useSpecMutation(exampleCreateSpec);
  const endpointDeleteMutation = useSpecMutation(endpointDeleteSpec);
  const endpointDuplicateMutation = useConnectMutation(endpointDuplicate, {
    onSuccess: invalidateCollectionListQuery,
  });

  return (
    <TreeItem
      textValue={endpoint.name}
      href={{
        from: workspaceRoute.id,
        to: '/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan',
        params: { endpointIdCan, exampleIdCan },
      }}
      isActive={match.params.exampleIdCan === exampleIdCan}
      childItems={exampleListQuery.data?.items ?? []}
      childItem={(_) => {
        const exampleIdCan = Ulid.construct(_.exampleId).toCanonical();
        return <ExampleItem id={exampleIdCan} endpointId={endpointId} example={_} />;
      }}
      expandButtonIsForced={!enabled}
      expandButtonOnPress={() => void setEnabled(true)}
    >
      {exampleListQuery.isLoading && (
        <Button variant='ghost' isDisabled className={tw`p-1`}>
          <FiRotateCw className={tw`size-3 animate-spin text-slate-500`} />
        </Button>
      )}

      <MethodBadge method={method} />

      <Text className='flex-1 truncate'>{endpoint.name}</Text>

      <MenuTrigger>
        <Button variant='ghost' className={tw`p-0.5`}>
          <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
        </Button>

        <Menu>
          <MenuItem
            onAction={() =>
              void exampleCreateMutation.mutate({
                endpointId,
                name: 'New Example',
              })
            }
          >
            Add Example
          </MenuItem>

          <MenuItem onAction={() => void endpointDuplicateMutation.mutate({ endpointId })}>Duplicate</MenuItem>

          <MenuItem
            variant='danger'
            onAction={() =>
              void endpointDeleteMutation.mutate({
                collectionId,
                endpointId,
                parentFolderId: parentFolderId!,
              })
            }
          >
            Delete
          </MenuItem>
        </Menu>
      </MenuTrigger>
    </TreeItem>
  );
};

interface ExampleItemProps {
  id: string;
  endpointId: Endpoint['endpointId'];
  example: ExampleListItem;
}

const ExampleItem = ({ id: exampleIdCan, endpointId, example }: ExampleItemProps) => {
  const match = useMatch({ strict: false });

  const endpointIdCan = Ulid.construct(endpointId).toCanonical();

  const invalidateCollectionListQuery = useInvalidateCollectionListQuery();
  const exampleDeleteMutation = useSpecMutation(exampleDeleteSpec);
  const exampleDuplicateMutation = useConnectMutation(exampleDuplicate, {
    onSuccess: invalidateCollectionListQuery,
  });

  const { exampleId } = example;

  return (
    <TreeItem
      textValue={example.name}
      href={{
        from: workspaceRoute.id,
        to: '/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan',
        params: { endpointIdCan, exampleIdCan },
      }}
      isActive={match.params.exampleIdCan === exampleIdCan}
    >
      <MdLightbulbOutline className={tw`size-4 text-violet-600`} />

      <Text className='flex-1 truncate'>{example.name}</Text>

      <MenuTrigger>
        <Button variant='ghost' className={tw`p-0.5`}>
          <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
        </Button>

        <Menu>
          <MenuItem onAction={() => void exampleDuplicateMutation.mutate({ exampleId })}>Duplicate</MenuItem>

          <MenuItem variant='danger' onAction={() => void exampleDeleteMutation.mutate({ endpointId, exampleId })}>
            Delete
          </MenuItem>
        </Menu>
      </MenuTrigger>
    </TreeItem>
  );
};
