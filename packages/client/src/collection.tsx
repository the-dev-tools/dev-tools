import { MessageInitShape } from '@bufbuild/protobuf';
import { getRouteApi, ToOptions, useMatchRoute, useNavigate, useRouteContext } from '@tanstack/react-router';
import { Match, pipe, Schema } from 'effect';
import { Ulid } from 'id128';
import { createContext, RefObject, useContext, useRef, useState } from 'react';
import { MenuTrigger, Text, Tree, useDragAndDrop } from 'react-aria-components';
import { FiFolder, FiMoreHorizontal } from 'react-icons/fi';
import { MdLightbulbOutline } from 'react-icons/md';
import { twJoin } from 'tailwind-merge';
import {
  Endpoint,
  EndpointCreateRequestSchema,
  EndpointListItem,
} from '@the-dev-tools/spec/collection/item/endpoint/v1/endpoint_pb';
import { ExampleListItem } from '@the-dev-tools/spec/collection/item/example/v1/example_pb';
import { Folder, FolderListItem } from '@the-dev-tools/spec/collection/item/folder/v1/folder_pb';
import { CollectionItem, ItemKind } from '@the-dev-tools/spec/collection/item/v1/item_pb';
import { Collection, CollectionListItem } from '@the-dev-tools/spec/collection/v1/collection_pb';
import { export$ } from '@the-dev-tools/spec/export/v1/export-ExportService_connectquery';
import {
  EndpointCreateEndpoint,
  EndpointDeleteEndpoint,
  EndpointDuplicateEndpoint,
  EndpointUpdateEndpoint,
} from '@the-dev-tools/spec/meta/collection/item/endpoint/v1/endpoint.endpoints.ts';
import {
  ExampleCreateEndpoint,
  ExampleDeleteEndpoint,
  ExampleDuplicateEndpoint,
  ExampleListEndpoint,
  ExampleUpdateEndpoint,
} from '@the-dev-tools/spec/meta/collection/item/example/v1/example.endpoints.ts';
import {
  FolderCreateEndpoint,
  FolderDeleteEndpoint,
  FolderUpdateEndpoint,
} from '@the-dev-tools/spec/meta/collection/item/folder/v1/folder.endpoints.ts';
import {
  CollectionItemListEndpoint,
  CollectionItemMoveEndpoint,
} from '@the-dev-tools/spec/meta/collection/item/v1/item.endpoints.ts';
import {
  CollectionDeleteEndpoint,
  CollectionListEndpoint,
  CollectionUpdateEndpoint,
} from '@the-dev-tools/spec/meta/collection/v1/collection.endpoints.ts';
import { MovePosition } from '@the-dev-tools/spec/resources/v1/resources_pb';
import { Button } from '@the-dev-tools/ui/button';
import { FolderOpenedIcon } from '@the-dev-tools/ui/icons';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { MethodBadge } from '@the-dev-tools/ui/method-badge';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { TreeItem, TreeItemLink, TreeItemProps } from '@the-dev-tools/ui/tree';
import { saveFile, useEscapePortal } from '@the-dev-tools/ui/utils';
import { useConnectMutation } from '~/api/connect-query';
import { useDLE, useMutate, useQuery } from '~data-client';
import { useOnEndpointDelete } from '~endpoint';

const workspaceRoute = getRouteApi('/_authorized/workspace/$workspaceIdCan');

interface CollectionListTreeContext {
  containerRef: RefObject<HTMLDivElement | null>;
  navigate?: boolean;
  showControls?: boolean;
}

const CollectionListTreeContext = createContext({} as CollectionListTreeContext);

class CollectionKey extends Schema.TaggedClass<CollectionKey>()('CollectionKey', {
  collectionId: Schema.Uint8ArrayFromBase64,
}) {}

class FolderKey extends Schema.TaggedClass<FolderKey>()('FolderKey', {
  collectionId: Schema.Uint8ArrayFromBase64,
  folderId: Schema.Uint8ArrayFromBase64,
  parentFolderId: pipe(Schema.Uint8ArrayFromBase64, Schema.optional),
}) {}

class EndpointKey extends Schema.TaggedClass<EndpointKey>()('EndpointKey', {
  collectionId: Schema.Uint8ArrayFromBase64,
  endpointId: Schema.Uint8ArrayFromBase64,
  exampleId: Schema.Uint8ArrayFromBase64,
  parentFolderId: pipe(Schema.Uint8ArrayFromBase64, Schema.optional),
}) {}

class ExampleKey extends Schema.TaggedClass<ExampleKey>()('ExampleKey', {
  collectionId: Schema.Uint8ArrayFromBase64,
  endpointId: Schema.Uint8ArrayFromBase64,
  exampleId: Schema.Uint8ArrayFromBase64,
}) {}

const TreeKey = Schema.Union(CollectionKey, FolderKey, EndpointKey, ExampleKey);

const getTreeKeyItemKind = (tag: typeof TreeKey.Type._tag) =>
  pipe(
    Match.value(tag),
    Match.when(EndpointKey._tag, () => ItemKind.ENDPOINT),
    Match.when(FolderKey._tag, () => ItemKind.FOLDER),
    Match.orElse(() => ItemKind.UNSPECIFIED),
  );

const getTreeKeyItemId = (key: EndpointKey | FolderKey) =>
  pipe(
    Match.value(key),
    Match.when({ _tag: EndpointKey._tag }, (_) => _.endpointId),
    Match.when({ _tag: FolderKey._tag }, (_) => _.folderId),
    Match.exhaustive,
  );

interface CollectionListTreeProps extends Omit<CollectionListTreeContext, 'containerRef'> {
  onAction?: (key: typeof TreeKey.Type) => void;
}

export const CollectionListTree = ({ onAction, ...context }: CollectionListTreeProps) => {
  const { dataClient } = useRouteContext({ from: '__root__' });
  const { workspaceId } = workspaceRoute.useLoaderData();

  const { items: collections } = useQuery(CollectionListEndpoint, { workspaceId });

  const ref = useRef<HTMLDivElement>(null);

  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) =>
      [...keys].map((key) => {
        const { _tag } = pipe(Schema.parseJson(TreeKey), Schema.decodeUnknownSync, (_) => _(key));
        return { [_tag]: '', key: key.toString() };
      }),

    shouldAcceptItemDrop: ({ dropPosition, key }, types) => {
      if (dropPosition !== 'on') return false;

      const { _tag: targetType } = pipe(Schema.parseJson(TreeKey), Schema.decodeUnknownSync, (_) => _(key));

      if (types.has(EndpointKey._tag) && targetType === FolderKey._tag) return true;

      if (types.has(FolderKey._tag)) {
        if (targetType === FolderKey._tag) return true;
        if (targetType === CollectionKey._tag) return true;
      }

      return false;
    },

    onItemDrop: async ({ items, target }) => {
      const [item] = items;
      if (target.dropPosition !== 'on' || !item || item.kind !== 'text' || items.length !== 1) return;

      const key = await pipe(Schema.parseJson(TreeKey), Schema.decodeUnknownSync, async (decode) =>
        pipe(await item.getText('key'), decode),
      );

      const targetKey = pipe(Schema.parseJson(TreeKey), Schema.decodeUnknownSync, (_) => _(target.key));

      if (key._tag !== FolderKey._tag && key._tag !== EndpointKey._tag) return;
      if (targetKey._tag !== CollectionKey._tag && targetKey._tag !== FolderKey._tag) return;

      void dataClient.fetch(CollectionItemMoveEndpoint, {
        collectionId: key.collectionId,
        itemId: getTreeKeyItemId(key),
        kind: getTreeKeyItemKind(key._tag),
        parentFolderId: key.parentFolderId!,
        targetCollectionId: targetKey.collectionId,
        ...(targetKey._tag === FolderKey._tag ? { targetParentFolderId: targetKey.folderId } : {}),
      });
    },
    onReorder: ({ keys, target }) => {
      const [keyMaybe] = keys;
      if (!keyMaybe || keys.size !== 1) return;

      const key = pipe(Schema.parseJson(TreeKey), Schema.decodeUnknownSync, (_) => _(keyMaybe));

      const targetKey = pipe(Schema.parseJson(TreeKey), Schema.decodeUnknownSync, (_) => _(target.key));

      const position = pipe(
        Match.value(target.dropPosition),
        Match.when('after', () => MovePosition.AFTER),
        Match.when('before', () => MovePosition.BEFORE),
        Match.orElse(() => MovePosition.UNSPECIFIED),
      );

      if (
        (key._tag === FolderKey._tag || key._tag === EndpointKey._tag) &&
        (targetKey._tag === FolderKey._tag || targetKey._tag === EndpointKey._tag)
      ) {
        void dataClient.fetch(CollectionItemMoveEndpoint, {
          collectionId: key.collectionId,
          itemId: getTreeKeyItemId(key),
          kind: getTreeKeyItemKind(key._tag),
          parentFolderId: key.parentFolderId!,
          position,
          targetItemId: getTreeKeyItemId(targetKey),
          targetKind: getTreeKeyItemKind(targetKey._tag),
        });
      }
    },

    renderDropIndicator: () => <div className={tw`relative z-10 h-0 w-full ring ring-violet-700`} />,
  });

  return (
    <CollectionListTreeContext.Provider value={{ ...context, containerRef: ref }}>
      <div className={tw`relative`} ref={ref}>
        <Tree
          aria-label='Collections'
          dragAndDropHooks={dragAndDropHooks}
          items={collections}
          onAction={
            onAction !== undefined
              ? (keyUnknown) => {
                  if (typeof keyUnknown !== 'string') return;
                  const key = pipe(Schema.parseJson(TreeKey), Schema.decodeUnknownSync, (_) => _(keyUnknown));
                  onAction(key);
                }
              : undefined!
          }
        >
          {(_) => {
            const collectionIdCan = Ulid.construct(_.collectionId).toCanonical();
            return <CollectionTree collection={_} id={collectionIdCan} />;
          }}
        </Tree>
      </div>
    </CollectionListTreeContext.Provider>
  );
};

interface CollectionTreeProps {
  collection: CollectionListItem;
  id: string;
}

const CollectionTree = ({ collection }: CollectionTreeProps) => {
  const { dataClient } = useRouteContext({ from: '__root__' });

  const navigate = useNavigate();

  const { containerRef, showControls } = useContext(CollectionListTreeContext);

  const { collectionId } = collection;
  const [enabled, setEnabled] = useState(false);

  const {
    data: { items },
    loading,
  } = useDLE(CollectionItemListEndpoint, enabled ? { collectionId } : null);
  const [collectionUpdate, collectionUpdateLoading] = useMutate(CollectionUpdateEndpoint);

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const escape = useEscapePortal(containerRef);

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => collectionUpdate({ collectionId, name: _ }),
    value: collection.name,
  });

  const childItems = (items ?? []).filter((_) => {
    if (_.kind !== ItemKind.ENDPOINT) return true;
    return !_.endpoint.hidden && !_.example.hidden;
  });

  return (
    <TreeItem
      childItem={mapCollectionItemTree(collectionId)}
      childItems={childItems}
      expandButtonIsForced={!enabled}
      expandButtonOnPress={() => void setEnabled(true)}
      id={pipe(new CollectionKey({ collectionId }), Schema.encodeSync(TreeKey), JSON.stringify)}
      loading={loading}
      textValue={collection.name}
      wrapperOnContextMenu={onContextMenu}
    >
      <Text className={twJoin(tw`flex-1 truncate`, isEditing && tw`opacity-0`)} ref={escape.ref}>
        {collection.name}
      </Text>

      {isEditing &&
        escape.render(
          <TextField
            aria-label='Collection name'
            className={tw`w-full`}
            inputClassName={tw`-my-1 py-1`}
            isDisabled={collectionUpdateLoading}
            {...textFieldProps}
          />,
        )}

      {showControls && (
        <MenuTrigger {...menuTriggerProps}>
          <Button className={tw`p-0.5`} variant='ghost'>
            <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
          </Button>

          <Menu {...menuProps}>
            <MenuItem onAction={() => void edit()}>Rename</MenuItem>

            <MenuItem
              onAction={async () => {
                const {
                  endpoint: { endpointId },
                  example: { exampleId },
                } = await dataClient.fetch(EndpointCreateEndpoint, {
                  collectionId,
                  method: 'GET',
                  name: 'New API call',
                });

                const endpointIdCan = Ulid.construct(endpointId).toCanonical();
                const exampleIdCan = Ulid.construct(exampleId).toCanonical();

                await navigate({
                  from: '/workspace/$workspaceIdCan',
                  to: '/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan',

                  params: { endpointIdCan, exampleIdCan },
                });
              }}
            >
              Add Request
            </MenuItem>

            <MenuItem onAction={() => dataClient.fetch(FolderCreateEndpoint, { collectionId, name: 'New folder' })}>
              Add Folder
            </MenuItem>

            <MenuItem onAction={() => dataClient.fetch(CollectionDeleteEndpoint, { collectionId })} variant='danger'>
              Delete
            </MenuItem>
          </Menu>
        </MenuTrigger>
      )}
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
          <FolderTree collectionId={collectionId} folder={_.folder!} id={folderIdCan} parentFolderId={parentFolderId} />
        );
      }),
      Match.when({ kind: ItemKind.ENDPOINT }, (_) => {
        const endpointIdCan = Ulid.construct(_.endpoint!.endpointId).toCanonical();
        return (
          <EndpointTree
            collectionId={collectionId}
            endpoint={_.endpoint!}
            example={_.example!}
            id={endpointIdCan}
            parentFolderId={parentFolderId}
          />
        );
      }),
      Match.orElse(() => null),
    );

interface FolderTreeProps {
  collectionId: Collection['collectionId'];
  folder: FolderListItem;
  id: string;
  parentFolderId: Folder['folderId'] | undefined;
}

const FolderTree = ({ collectionId, folder: { folderId, ...folder }, parentFolderId }: FolderTreeProps) => {
  const { dataClient } = useRouteContext({ from: '__root__' });

  const navigate = useNavigate();

  const { containerRef, showControls } = useContext(CollectionListTreeContext);

  const [enabled, setEnabled] = useState(false);

  const {
    data: { items },
    loading,
  } = useDLE(CollectionItemListEndpoint, enabled ? { collectionId, parentFolderId: folderId } : null);

  const [folderUpdate, folderUpdateLoading] = useMutate(FolderUpdateEndpoint);

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const escape = useEscapePortal(containerRef);

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) =>
      folderUpdate({
        folderId,
        name: _,
        parentFolderId: parentFolderId!,
      }),
    value: folder.name,
  });

  const childItems = (items ?? []).filter((_) => {
    if (_.kind !== ItemKind.ENDPOINT) return true;
    return !_.endpoint.hidden && !_.example.hidden;
  });

  return (
    <TreeItem
      childItem={mapCollectionItemTree(collectionId, folderId)}
      childItems={childItems}
      expandButtonIsForced={!enabled}
      expandButtonOnPress={() => void setEnabled(true)}
      id={pipe(new FolderKey({ collectionId, folderId, parentFolderId }), Schema.encodeSync(TreeKey), JSON.stringify)}
      loading={loading}
      textValue={folder.name}
      wrapperOnContextMenu={onContextMenu}
    >
      {({ isExpanded }) => (
        <>
          {isExpanded ? (
            <FolderOpenedIcon className={tw`size-4 text-slate-500`} />
          ) : (
            <FiFolder className={tw`size-4 text-slate-500`} />
          )}

          <Text className={twJoin(tw`flex-1 truncate`, isEditing && tw`opacity-0`)} ref={escape.ref}>
            {folder.name}
          </Text>

          {isEditing &&
            escape.render(
              <TextField
                aria-label='Folder name'
                className={tw`w-full`}
                inputClassName={tw`-my-1 py-1`}
                isDisabled={folderUpdateLoading}
                {...textFieldProps}
              />,
            )}

          {showControls && (
            <MenuTrigger {...menuTriggerProps}>
              <Button className={tw`p-0.5`} variant='ghost'>
                <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
              </Button>

              <Menu {...menuProps}>
                <MenuItem onAction={() => void edit()}>Rename</MenuItem>

                <MenuItem
                  onAction={async () => {
                    const {
                      endpoint: { endpointId },
                      example: { exampleId },
                    } = await dataClient.fetch(EndpointCreateEndpoint, {
                      collectionId,
                      method: 'GET',
                      name: 'New API call',
                      parentFolderId: folderId,
                    });

                    const endpointIdCan = Ulid.construct(endpointId).toCanonical();
                    const exampleIdCan = Ulid.construct(exampleId).toCanonical();

                    await navigate({
                      from: '/workspace/$workspaceIdCan',
                      to: '/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan',

                      params: { endpointIdCan, exampleIdCan },
                    });
                  }}
                >
                  Add Request
                </MenuItem>

                <MenuItem
                  onAction={() =>
                    dataClient.fetch(FolderCreateEndpoint, {
                      collectionId,
                      name: 'New folder',
                      parentFolderId: folderId,
                    })
                  }
                >
                  Add Folder
                </MenuItem>

                <MenuItem onAction={() => dataClient.fetch(FolderDeleteEndpoint, { folderId })} variant='danger'>
                  Delete
                </MenuItem>
              </Menu>
            </MenuTrigger>
          )}
        </>
      )}
    </TreeItem>
  );
};

interface EndpointTreeProps {
  collectionId: Collection['collectionId'];
  endpoint: EndpointListItem;
  example: ExampleListItem;
  id: string;
  parentFolderId?: Uint8Array | undefined;
}

const EndpointTree = ({ collectionId, endpoint, example, id: endpointIdCan, parentFolderId }: EndpointTreeProps) => {
  const { dataClient } = useRouteContext({ from: '__root__' });

  const { endpointId, method, name } = endpoint;
  const { exampleId, lastResponseId } = example;

  const matchRoute = useMatchRoute();
  const navigate = useNavigate();

  const onEndpointDelete = useOnEndpointDelete();

  const { workspaceId } = workspaceRoute.useLoaderData();

  const { containerRef, navigate: toNavigate = false, showControls } = useContext(CollectionListTreeContext);

  const exampleIdCan = Ulid.construct(exampleId).toCanonical();
  const lastResponseIdCan = lastResponseId && Ulid.construct(lastResponseId).toCanonical();

  const [enabled, setEnabled] = useState(false);

  const {
    data: { items },
    loading,
  } = useDLE(ExampleListEndpoint, enabled ? { endpointId } : null);

  const [endpointUpdate, endpointUpdateLoading] = useMutate(EndpointUpdateEndpoint);

  // TODO: switch to Data Client Endpoint
  const exportMutation = useConnectMutation(export$);

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const escape = useEscapePortal(containerRef);

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => endpointUpdate({ endpointId, name: _ }),
    value: endpoint.name,
  });

  const route = {
    from: '/workspace/$workspaceIdCan',
    params: { endpointIdCan, exampleIdCan },
    search: { responseIdCan: lastResponseIdCan },
    to: '/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan',
  } satisfies ToOptions;

  const childItems = (items ?? []).filter((_) => !_.hidden);

  const content = (
    <>
      {method && <MethodBadge method={method} />}

      <Text className={twJoin(tw`flex-1 truncate`, isEditing && tw`opacity-0`)} ref={escape.ref}>
        {name}
      </Text>

      {isEditing &&
        escape.render(
          <TextField
            aria-label='Endpoint name'
            className={tw`w-full`}
            inputClassName={tw`-my-1 py-1`}
            isDisabled={endpointUpdateLoading}
            {...textFieldProps}
          />,
        )}

      {showControls && (
        <MenuTrigger {...menuTriggerProps}>
          <Button className={tw`p-0.5`} variant='ghost'>
            <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
          </Button>

          <Menu {...menuProps}>
            <MenuItem onAction={() => void edit()}>Rename</MenuItem>

            <MenuItem
              onAction={async () => {
                const { exampleId } = await dataClient.fetch(ExampleCreateEndpoint, {
                  endpointId,
                  name: 'New Example',
                });

                const exampleIdCan = Ulid.construct(exampleId).toCanonical();

                await navigate({
                  from: '/workspace/$workspaceIdCan',
                  to: '/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan',

                  params: { endpointIdCan, exampleIdCan },
                });
              }}
            >
              Add Example
            </MenuItem>

            <MenuItem
              onAction={() => {
                const input: MessageInitShape<typeof EndpointCreateRequestSchema> = { collectionId, endpointId };
                if (parentFolderId) input.parentFolderId = parentFolderId;
                return dataClient.fetch(EndpointDuplicateEndpoint, input);
              }}
            >
              Duplicate
            </MenuItem>

            <MenuItem
              onAction={async () => {
                const { data, name } = await exportMutation.mutateAsync({ exampleIds: [exampleId], workspaceId });
                saveFile({ blobParts: [data], name });
              }}
            >
              Export
            </MenuItem>

            <MenuItem
              onAction={async () => {
                await onEndpointDelete({ endpointId, exampleId });
                await dataClient.fetch(EndpointDeleteEndpoint, { endpointId });
              }}
              variant='danger'
            >
              Delete
            </MenuItem>
          </Menu>
        </MenuTrigger>
      )}
    </>
  );

  const props = {
    childItem: (_) => {
      const exampleIdCan = Ulid.construct(_.exampleId).toCanonical();
      return <ExampleItem collectionId={collectionId} endpointId={endpointId} example={_} id={exampleIdCan} />;
    },
    childItems: childItems,
    expandButtonIsForced: !enabled,
    expandButtonOnPress: () => void setEnabled(true),
    id: pipe(
      new EndpointKey({ collectionId, endpointId, exampleId, parentFolderId }),
      Schema.encodeSync(TreeKey),
      JSON.stringify,
    ),
    isActive: toNavigate && matchRoute(route) !== false,
    loading: loading,
    textValue: name,
    wrapperOnContextMenu: onContextMenu,
  } satisfies TreeItemProps<ExampleListItem>;

  return toNavigate ? (
    <TreeItemLink {...props} {...route}>
      {content}
    </TreeItemLink>
  ) : (
    <TreeItem {...props}>{content}</TreeItem>
  );
};

interface ExampleItemProps {
  collectionId: Collection['collectionId'];
  endpointId: Endpoint['endpointId'];
  example: ExampleListItem;
  id: string;
}

const ExampleItem = ({ collectionId, endpointId, example, id: exampleIdCan }: ExampleItemProps) => {
  const { dataClient } = useRouteContext({ from: '__root__' });

  const { exampleId, lastResponseId, name } = example;

  const endpointIdCan = Ulid.construct(endpointId).toCanonical();
  const lastResponseIdCan = lastResponseId && Ulid.construct(lastResponseId).toCanonical();

  const matchRoute = useMatchRoute();

  const onEndpointDelete = useOnEndpointDelete();

  const { workspaceId } = workspaceRoute.useLoaderData();

  const { containerRef, navigate: toNavigate = false, showControls } = useContext(CollectionListTreeContext);

  const [exampleUpdate, exampleUpdateLoading] = useMutate(ExampleUpdateEndpoint);

  // TODO: switch to Data Client Endpoint
  const exportMutation = useConnectMutation(export$);

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const escape = useEscapePortal(containerRef);

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => exampleUpdate({ exampleId, name: _ }),
    value: name,
  });

  const route = {
    from: '/workspace/$workspaceIdCan',
    params: { endpointIdCan, exampleIdCan },
    search: { responseIdCan: lastResponseIdCan },
    to: '/workspace/$workspaceIdCan/endpoint/$endpointIdCan/example/$exampleIdCan',
  } satisfies ToOptions;

  const content = (
    <>
      <MdLightbulbOutline className={tw`size-4 text-violet-600`} />

      <Text className={twJoin(tw`flex-1 truncate`, isEditing && tw`opacity-0`)} ref={escape.ref}>
        {name}
      </Text>

      {isEditing &&
        escape.render(
          <TextField
            aria-label='Example name'
            className={tw`w-full`}
            inputClassName={tw`-my-1 py-1`}
            isDisabled={exampleUpdateLoading}
            {...textFieldProps}
          />,
        )}

      {showControls && (
        <MenuTrigger {...menuTriggerProps}>
          <Button className={tw`p-0.5`} variant='ghost'>
            <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
          </Button>

          <Menu {...menuProps}>
            <MenuItem onAction={() => void edit()}>Rename</MenuItem>

            <MenuItem onAction={() => dataClient.fetch(ExampleDuplicateEndpoint, { exampleId })}>Duplicate</MenuItem>

            <MenuItem
              onAction={async () => {
                const { data, name } = await exportMutation.mutateAsync({ exampleIds: [exampleId], workspaceId });
                saveFile({ blobParts: [data], name });
              }}
            >
              Export
            </MenuItem>

            <MenuItem
              onAction={async () => {
                await onEndpointDelete({ endpointId, exampleId });
                await dataClient.fetch(ExampleDeleteEndpoint, { exampleId });
              }}
              variant='danger'
            >
              Delete
            </MenuItem>
          </Menu>
        </MenuTrigger>
      )}
    </>
  );

  const props = {
    id: pipe(new ExampleKey({ collectionId, endpointId, exampleId }), Schema.encodeSync(TreeKey), JSON.stringify),
    isActive: toNavigate && matchRoute(route) !== false,
    textValue: name,
    wrapperOnContextMenu: onContextMenu,
  } satisfies TreeItemProps<object>;

  return toNavigate ? (
    <TreeItemLink {...props} {...route}>
      {content}
    </TreeItemLink>
  ) : (
    <TreeItem {...props}>{content}</TreeItem>
  );
};
