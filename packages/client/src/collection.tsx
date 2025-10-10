import { MessageInitShape } from '@bufbuild/protobuf';
import { ToOptions, useMatchRoute, useNavigate } from '@tanstack/react-router';
import CodeMirror from '@uiw/react-codemirror';
import { Match, pipe, Schema } from 'effect';
import { Ulid } from 'id128';
import { createContext, ReactNode, RefObject, useContext, useRef, useState } from 'react';
import {
  Dialog,
  DialogTrigger,
  MenuTrigger,
  SubmenuTrigger,
  Text,
  Tree,
  TreeProps,
  useDragAndDrop,
} from 'react-aria-components';
import { FiFolder, FiMoreHorizontal, FiX } from 'react-icons/fi';
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
import {
  EndpointCreateEndpoint,
  EndpointDeleteEndpoint,
  EndpointDuplicateEndpoint,
  EndpointUpdateEndpoint,
} from '@the-dev-tools/spec/data-client/collection/item/endpoint/v1/endpoint.endpoints.ts';
import {
  ExampleCreateEndpoint,
  ExampleDeleteEndpoint,
  ExampleDuplicateEndpoint,
  ExampleListEndpoint,
  ExampleMoveEndpoint,
  ExampleUpdateEndpoint,
} from '@the-dev-tools/spec/data-client/collection/item/example/v1/example.endpoints.ts';
import {
  FolderCreateEndpoint,
  FolderDeleteEndpoint,
  FolderUpdateEndpoint,
} from '@the-dev-tools/spec/data-client/collection/item/folder/v1/folder.endpoints.ts';
import {
  CollectionItemListEndpoint,
  CollectionItemMoveEndpoint,
} from '@the-dev-tools/spec/data-client/collection/item/v1/item.endpoints.ts';
import {
  CollectionDeleteEndpoint,
  CollectionListEndpoint,
  CollectionMoveEndpoint,
  CollectionUpdateEndpoint,
} from '@the-dev-tools/spec/data-client/collection/v1/collection.endpoints.ts';
import { CollectionListItemEntity } from '@the-dev-tools/spec/data-client/collection/v1/collection.entities.js';
import { export$, exportCurl } from '@the-dev-tools/spec/export/v1/export-ExportService_connectquery';
import { MovePosition } from '@the-dev-tools/spec/resource/v1/resource_pb';
import { Button } from '@the-dev-tools/ui/button';
import { FolderOpenedIcon } from '@the-dev-tools/ui/icons';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { MethodBadge } from '@the-dev-tools/ui/method-badge';
import { Modal } from '@the-dev-tools/ui/modal';
import { DropIndicatorHorizontal } from '@the-dev-tools/ui/reorder';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { TreeItem, TreeItemLink, TreeItemProps } from '@the-dev-tools/ui/tree';
import { saveFile, useEscapePortal } from '@the-dev-tools/ui/utils';
import { useConnectMutation } from '~/api/connect-query';
import { useDLE, useEndpointProps, useMutate, useQuery } from '~data-client';
import { useOnEndpointDelete } from '~endpoint';
import { requestRouteApi, rootRouteApi, workspaceRouteApi } from '~routes';

interface CollectionListTreeContext {
  containerRef: RefObject<HTMLDivElement | null>;
  navigate?: boolean;
  showControls?: boolean;
}

const CollectionListTreeContext = createContext({} as CollectionListTreeContext);

export class CollectionKey extends Schema.TaggedClass<CollectionKey>()('CollectionKey', {
  collectionId: Schema.Uint8ArrayFromBase64,
}) {}

export class FolderKey extends Schema.TaggedClass<FolderKey>()('FolderKey', {
  collectionId: Schema.Uint8ArrayFromBase64,
  folderId: Schema.Uint8ArrayFromBase64,
  parentFolderId: pipe(Schema.Uint8ArrayFromBase64, Schema.optional),
}) {}

export class EndpointKey extends Schema.TaggedClass<EndpointKey>()('EndpointKey', {
  collectionId: Schema.Uint8ArrayFromBase64,
  endpointId: Schema.Uint8ArrayFromBase64,
  exampleId: Schema.Uint8ArrayFromBase64,
  parentFolderId: pipe(Schema.Uint8ArrayFromBase64, Schema.optional),
}) {}

export class ExampleKey extends Schema.TaggedClass<ExampleKey>()('ExampleKey', {
  collectionId: Schema.Uint8ArrayFromBase64,
  endpointId: Schema.Uint8ArrayFromBase64,
  exampleId: Schema.Uint8ArrayFromBase64,
}) {}

export const TreeKey = Schema.Union(CollectionKey, FolderKey, EndpointKey, ExampleKey);

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

const useOnCollectionDelete = () => {
  const { dataClient } = rootRouteApi.useRouteContext();
  const endpointProps = useEndpointProps();
  const onEndpointDelete = useOnEndpointDelete();
  const onFolderDelete = useOnFolderDelete();
  const onCollectionrDelete = async (collectionId: Uint8Array) => {
    const state = dataClient.controller.getState();
    const {
      data: { items },
    } = dataClient.controller.getResponse(
      CollectionItemListEndpoint,
      { ...endpointProps, input: { collectionId } },
      state,
    );
    for (const _ of items ?? []) {
      if (_.kind === ItemKind.FOLDER) await onFolderDelete(collectionId, _.folder.folderId);

      if (_.kind === ItemKind.ENDPOINT)
        await onEndpointDelete({ endpointId: _.endpoint.endpointId, exampleId: _.example.exampleId });
    }
  };
  return onCollectionrDelete;
};

const useOnFolderDelete = () => {
  const { dataClient } = rootRouteApi.useRouteContext();
  const endpointProps = useEndpointProps();
  const onEndpointDelete = useOnEndpointDelete();
  const onFolderDelete = async (collectionId: Uint8Array, folderId: Uint8Array) => {
    const state = dataClient.controller.getState();
    const {
      data: { items },
    } = dataClient.controller.getResponse(
      CollectionItemListEndpoint,
      { ...endpointProps, input: { collectionId, parentFolderId: folderId } },
      state,
    );
    for (const _ of items ?? []) {
      if (_.kind === ItemKind.FOLDER) await onFolderDelete(collectionId, _.folder.folderId);

      if (_.kind === ItemKind.ENDPOINT)
        await onEndpointDelete({ endpointId: _.endpoint.endpointId, exampleId: _.example.exampleId });
    }
  };
  return onFolderDelete;
};

interface CollectionListTreeProps
  extends Omit<CollectionListTreeContext, 'containerRef'>,
    Pick<TreeProps<CollectionListItemEntity>, 'onSelectionChange' | 'selectedKeys' | 'selectionMode'> {
  onAction?: (key: typeof TreeKey.Type) => void;
}

export const CollectionListTree = ({
  onAction,
  onSelectionChange,
  selectedKeys,
  selectionMode,
  ...context
}: CollectionListTreeProps) => {
  const { dataClient } = rootRouteApi.useRouteContext();
  const { workspaceId } = workspaceRouteApi.useLoaderData();

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

      const sourceCanMove = types.has(EndpointKey._tag) || types.has(FolderKey._tag);
      const targetCanAccept = targetType === FolderKey._tag || targetType === CollectionKey._tag;

      return sourceCanMove && targetCanAccept;
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
        ...(targetKey._tag === FolderKey._tag && { targetParentFolderId: targetKey.folderId }),
      });
    },
    onReorder: ({ keys, target }) => {
      const [keyMaybe] = keys;
      if (!keyMaybe || keys.size !== 1) return;

      const key = pipe(Schema.parseJson(TreeKey), Schema.decodeUnknownSync, (_) => _(keyMaybe));

      const targetKey = pipe(Schema.parseJson(TreeKey), Schema.decodeUnknownSync, (_) => _(target.key));

      if (Schema.equivalence(TreeKey)(key, targetKey)) return;

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

      if (key._tag === ExampleKey._tag && targetKey._tag === ExampleKey._tag) {
        void dataClient.fetch(ExampleMoveEndpoint, {
          endpointId: key.endpointId,
          exampleId: key.exampleId,
          position,
          targetExampleId: targetKey.exampleId,
        });
      }

      if (key._tag === CollectionKey._tag && targetKey._tag === CollectionKey._tag) {
        void dataClient.fetch(CollectionMoveEndpoint, {
          collectionId: key.collectionId,
          position,
          targetCollectionId: targetKey.collectionId,
          workspaceId,
        });
      }
    },

    renderDropIndicator: () => <DropIndicatorHorizontal />,
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
          onSelectionChange={onSelectionChange!}
          selectedKeys={selectedKeys!}
          selectionMode={selectionMode!}
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
  const { dataClient } = rootRouteApi.useRouteContext();

  const navigate = useNavigate();

  const { containerRef, showControls } = useContext(CollectionListTreeContext);

  const { collectionId } = collection;
  const [enabled, setEnabled] = useState(false);

  const {
    data: { items },
    loading,
  } = useDLE(CollectionItemListEndpoint, enabled ? { collectionId } : null);
  const [collectionUpdate, collectionUpdateLoading] = useMutate(CollectionUpdateEndpoint);
  const onCollectionDelete = useOnCollectionDelete();

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const { escapeRef, escapeRender } = useEscapePortal(containerRef);

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
      id={pipe(new CollectionKey({ collectionId }), Schema.encodeSync(TreeKey), JSON.stringify)}
      isLoading={loading}
      item={mapCollectionItemTree(collectionId)}
      items={childItems}
      onContextMenu={onContextMenu}
      onExpand={() => void setEnabled(true)}
      textValue={collection.name}
    >
      <Text className={twJoin(tw`flex-1 truncate`, isEditing && tw`opacity-0`)} ref={escapeRef}>
        {collection.name}
      </Text>

      {isEditing &&
        escapeRender(
          <TextInputField
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
                  from: workspaceRouteApi.id,
                  to: requestRouteApi.id,

                  params: { endpointIdCan, exampleIdCan },
                });
              }}
            >
              Add Request
            </MenuItem>

            <MenuItem onAction={() => dataClient.fetch(FolderCreateEndpoint, { collectionId, name: 'New folder' })}>
              Add Folder
            </MenuItem>

            <MenuItem
              onAction={async () => {
                await onCollectionDelete(collectionId);
                await dataClient.fetch(CollectionDeleteEndpoint, { collectionId });
              }}
              variant='danger'
            >
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
  const { dataClient } = rootRouteApi.useRouteContext();

  const navigate = useNavigate();

  const { containerRef, showControls } = useContext(CollectionListTreeContext);

  const [enabled, setEnabled] = useState(false);

  const {
    data: { items },
    loading,
  } = useDLE(CollectionItemListEndpoint, enabled ? { collectionId, parentFolderId: folderId } : null);

  const [folderUpdate, folderUpdateLoading] = useMutate(FolderUpdateEndpoint);
  const onFolderDelete = useOnFolderDelete();

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const { escapeRef, escapeRender } = useEscapePortal(containerRef);

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
      id={pipe(new FolderKey({ collectionId, folderId, parentFolderId }), Schema.encodeSync(TreeKey), JSON.stringify)}
      isLoading={loading}
      item={mapCollectionItemTree(collectionId, folderId)}
      items={childItems}
      onContextMenu={onContextMenu}
      onExpand={() => void setEnabled(true)}
      textValue={folder.name}
    >
      {({ isExpanded }) => (
        <>
          {isExpanded ? (
            <FolderOpenedIcon className={tw`size-4 text-slate-500`} />
          ) : (
            <FiFolder className={tw`size-4 text-slate-500`} />
          )}

          <Text className={twJoin(tw`flex-1 truncate`, isEditing && tw`opacity-0`)} ref={escapeRef}>
            {folder.name}
          </Text>

          {isEditing &&
            escapeRender(
              <TextInputField
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
                      from: workspaceRouteApi.id,
                      to: requestRouteApi.id,

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

                <MenuItem
                  onAction={async () => {
                    await onFolderDelete(collectionId, folderId);
                    await dataClient.fetch(FolderDeleteEndpoint, { folderId });
                  }}
                  variant='danger'
                >
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
  const { dataClient } = rootRouteApi.useRouteContext();

  const { endpointId, method, name } = endpoint;
  const { exampleId, lastResponseId } = example;

  const matchRoute = useMatchRoute();
  const navigate = useNavigate();

  const onEndpointDelete = useOnEndpointDelete();

  const { workspaceId } = workspaceRouteApi.useLoaderData();

  const { containerRef, navigate: toNavigate = false, showControls } = useContext(CollectionListTreeContext);

  const exampleIdCan = Ulid.construct(exampleId).toCanonical();
  const lastResponseIdCan = lastResponseId && Ulid.construct(lastResponseId).toCanonical();

  const [modal, setModal] = useState<ReactNode>(null);

  const [enabled, setEnabled] = useState(false);

  const {
    data: { items },
    loading,
  } = useDLE(ExampleListEndpoint, enabled ? { endpointId } : null);

  const [endpointUpdate, endpointUpdateLoading] = useMutate(EndpointUpdateEndpoint);

  // TODO: switch to Data Client Endpoint
  const exportMutation = useConnectMutation(export$);
  const exportCurlMutation = useConnectMutation(exportCurl);

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const { escapeRef, escapeRender } = useEscapePortal(containerRef);

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => endpointUpdate({ endpointId, name: _ }),
    value: endpoint.name,
  });

  const route = {
    from: workspaceRouteApi.id,
    params: { endpointIdCan, exampleIdCan },
    search: { responseIdCan: lastResponseIdCan },
    to: requestRouteApi.id,
  } satisfies ToOptions;

  const childItems = (items ?? []).filter((_) => !_.hidden);

  const content = (
    <>
      {modal}

      {method && <MethodBadge method={method} />}

      <Text className={twJoin(tw`flex-1 truncate`, isEditing && tw`opacity-0`)} ref={escapeRef}>
        {name}
      </Text>

      {isEditing &&
        escapeRender(
          <TextInputField
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
                  from: workspaceRouteApi.id,
                  to: requestRouteApi.id,

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

            <SubmenuTrigger>
              <MenuItem>Export</MenuItem>
              <Menu>
                <MenuItem
                  onAction={async () => {
                    const { data, name } = await exportMutation.mutateAsync({ exampleIds: [exampleId], workspaceId });
                    saveFile({ blobParts: [data], name });
                  }}
                >
                  YAML (DevTools)
                </MenuItem>

                <MenuItem
                  onAction={async () => {
                    const { data } = await exportCurlMutation.mutateAsync({ exampleIds: [exampleId], workspaceId });
                    setModal(
                      <DialogTrigger isOpen onOpenChange={() => void setModal(null)}>
                        <Modal size='sm'>
                          <Dialog className={tw`flex h-full flex-col gap-4 p-6`}>
                            <div className={tw`flex items-center justify-between`}>
                              <div className={tw`text-xl leading-6 font-semibold tracking-tighter text-slate-800`}>
                                cURL export
                              </div>

                              <Button className={tw`p-1`} onPress={() => void setModal(null)} variant='ghost'>
                                <FiX className={tw`size-5 text-slate-500`} />
                              </Button>
                            </div>

                            <CodeMirror className={tw`flex-1`} height='100%' readOnly value={data} />
                          </Dialog>
                        </Modal>
                      </DialogTrigger>,
                    );
                  }}
                >
                  cURL
                </MenuItem>
              </Menu>
            </SubmenuTrigger>

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
    children: content,
    className: toNavigate && matchRoute(route) !== false ? tw`bg-slate-200` : '',
    id: pipe(
      new EndpointKey({ collectionId, endpointId, exampleId, parentFolderId }),
      Schema.encodeSync(TreeKey),
      JSON.stringify,
    ),
    isLoading: loading,
    item: (_) => {
      const exampleIdCan = Ulid.construct(_.exampleId).toCanonical();
      return <ExampleItem collectionId={collectionId} endpointId={endpointId} example={_} id={exampleIdCan} />;
    },
    items: childItems,
    onContextMenu: onContextMenu,
    onExpand: () => void setEnabled(true),
    textValue: name,
  } satisfies TreeItemProps<ExampleListItem>;

  return toNavigate ? <TreeItemLink {...props} {...route} /> : <TreeItem {...props} />;
};

interface ExampleItemProps {
  collectionId: Collection['collectionId'];
  endpointId: Endpoint['endpointId'];
  example: ExampleListItem;
  id: string;
}

const ExampleItem = ({ collectionId, endpointId, example, id: exampleIdCan }: ExampleItemProps) => {
  const { dataClient } = rootRouteApi.useRouteContext();

  const { exampleId, lastResponseId, name } = example;

  const endpointIdCan = Ulid.construct(endpointId).toCanonical();
  const lastResponseIdCan = lastResponseId && Ulid.construct(lastResponseId).toCanonical();

  const matchRoute = useMatchRoute();

  const onEndpointDelete = useOnEndpointDelete();

  const { workspaceId } = workspaceRouteApi.useLoaderData();

  const { containerRef, navigate: toNavigate = false, showControls } = useContext(CollectionListTreeContext);

  const [exampleUpdate, exampleUpdateLoading] = useMutate(ExampleUpdateEndpoint);

  // TODO: switch to Data Client Endpoint
  const exportMutation = useConnectMutation(export$);

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const { escapeRef, escapeRender } = useEscapePortal(containerRef);

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => exampleUpdate({ exampleId, name: _ }),
    value: name,
  });

  const route = {
    from: workspaceRouteApi.id,
    params: { endpointIdCan, exampleIdCan },
    search: { responseIdCan: lastResponseIdCan },
    to: requestRouteApi.id,
  } satisfies ToOptions;

  const content = (
    <>
      <MdLightbulbOutline className={tw`size-4 text-violet-600`} />

      <Text className={twJoin(tw`flex-1 truncate`, isEditing && tw`opacity-0`)} ref={escapeRef}>
        {name}
      </Text>

      {isEditing &&
        escapeRender(
          <TextInputField
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
    children: content,
    className: toNavigate && matchRoute(route) !== false ? tw`bg-slate-200` : '',
    id: pipe(new ExampleKey({ collectionId, endpointId, exampleId }), Schema.encodeSync(TreeKey), JSON.stringify),
    onContextMenu: onContextMenu,
    textValue: name,
  } satisfies TreeItemProps<object>;

  return toNavigate ? <TreeItemLink {...props} {...route} /> : <TreeItem {...props} />;
};
