import { create } from '@bufbuild/protobuf';
import { eq, isUndefined, useLiveQuery } from '@tanstack/react-db';
import { ToOptions, useMatchRoute, useNavigate } from '@tanstack/react-router';
import CodeMirror from '@uiw/react-codemirror';
import { Match, pipe } from 'effect';
import { Ulid } from 'id128';
import { createContext, RefObject, useContext, useMemo, useRef } from 'react';
import {
  Dialog,
  Heading,
  MenuTrigger,
  SubmenuTrigger,
  Text,
  Tree,
  TreeProps,
  useDragAndDrop,
} from 'react-aria-components';
import { FiFolder, FiMoreHorizontal, FiX } from 'react-icons/fi';
import { twJoin } from 'tailwind-merge';
import { ExportService } from '@the-dev-tools/spec/buf/api/export/v1/export_pb';
import {
  File,
  FileKind,
  FileSchema,
  FileUpdate_ParentIdUnion_Kind,
  FolderSchema,
} from '@the-dev-tools/spec/buf/api/file_system/v1/file_system_pb';
import { FlowSchema, FlowService } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { HttpDeltaSchema, HttpMethod, HttpSchema, HttpService } from '@the-dev-tools/spec/buf/api/http/v1/http_pb';
import { FileCollectionSchema, FolderCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/file_system';
import { FlowCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { HttpCollectionSchema, HttpDeltaCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { Button } from '@the-dev-tools/ui/button';
import { FlowsIcon, FolderOpenedIcon } from '@the-dev-tools/ui/icons';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { MethodBadge } from '@the-dev-tools/ui/method-badge';
import { Modal, useProgrammaticModal } from '@the-dev-tools/ui/modal';
import { DropIndicatorHorizontal } from '@the-dev-tools/ui/reorder';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { TreeItem, TreeItemLink, TreeItemProps } from '@the-dev-tools/ui/tree';
import { saveFile, useEscapePortal } from '@the-dev-tools/ui/utils';
import { useApiCollection } from '~/api';
import { useConnectMutation } from '~/api/connect-query';
import { flowLayoutRouteApi, httpDeltaRouteApi, httpRouteApi, workspaceRouteApi } from '~/routes';
import { useDeltaState } from '~/utils/delta';
import { getNextOrder, handleCollectionReorder } from '~/utils/order';
import { pick } from '~/utils/tanstack-db';

interface FileCreateMenuProps {
  parentFolderId?: Uint8Array;
}

export const FileCreateMenu = ({ parentFolderId }: FileCreateMenuProps) => {
  const navigate = useNavigate();

  const { workspaceId } = workspaceRouteApi.useLoaderData();

  const fileCollection = useApiCollection(FileCollectionSchema);
  const folderCollection = useApiCollection(FolderCollectionSchema);
  const httpCollection = useApiCollection(HttpCollectionSchema);
  const flowCollection = useApiCollection(FlowCollectionSchema);

  const insertFile = async (props: Pick<File, 'fileId' | 'kind'>) =>
    fileCollection.utils.insert({
      ...props,
      ...(parentFolderId && { parentId: parentFolderId }),
      order: await getNextOrder(fileCollection),
      workspaceId,
    });

  return (
    <Menu>
      <MenuItem
        onAction={() => {
          const folderId = Ulid.generate().bytes;
          folderCollection.utils.insert({ folderId, name: 'New folder' });
          void insertFile({ fileId: folderId, kind: FileKind.FOLDER });
        }}
      >
        Folder
      </MenuItem>

      <MenuItem
        onAction={async () => {
          const httpUlid = Ulid.generate();
          httpCollection.utils.insert({ httpId: httpUlid.bytes, method: HttpMethod.GET, name: 'New HTTP request' });
          await insertFile({ fileId: httpUlid.bytes, kind: FileKind.HTTP });
          await navigate({
            from: workspaceRouteApi.id,
            params: { httpIdCan: httpUlid.toCanonical() },
            to: httpRouteApi.id,
          });
        }}
      >
        HTTP request
      </MenuItem>

      <MenuItem
        onAction={async () => {
          const flowUlid = Ulid.generate();
          flowCollection.utils.insert({ flowId: flowUlid.bytes, name: 'New flow', workspaceId });
          await insertFile({ fileId: flowUlid.bytes, kind: FileKind.FLOW });
          await navigate({
            from: workspaceRouteApi.id,
            params: { flowIdCan: flowUlid.toCanonical() },
            to: flowLayoutRouteApi.id,
          });
        }}
      >
        Flow
      </MenuItem>
    </Menu>
  );
};

interface FileTreeContext {
  containerRef: RefObject<HTMLDivElement | null>;
  navigate?: boolean;
  showControls?: boolean;
}

const FileTreeContext = createContext({} as FileTreeContext);

interface FileTreeProps
  extends Omit<FileTreeContext, 'containerRef'>,
    Pick<TreeProps<object>, 'onAction' | 'onSelectionChange' | 'selectedKeys' | 'selectionMode'> {}

export const FileTree = ({ onAction, onSelectionChange, selectedKeys, selectionMode, ...context }: FileTreeProps) => {
  const { workspaceId } = workspaceRouteApi.useLoaderData();

  const fileCollection = useApiCollection(FileCollectionSchema);

  const { data: files } = useLiveQuery(
    (_) =>
      _.from({ file: fileCollection })
        .where((_) => eq(_.file.workspaceId, workspaceId))
        .where((_) => isUndefined(_.file.parentId))
        .orderBy((_) => _.file.order)
        .select((_) => pick(_.file, 'fileId', 'order')),
    [fileCollection, workspaceId],
  );

  const ref = useRef<HTMLDivElement>(null);

  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) =>
      [...keys].map((key) => {
        const kind = pipe(
          key.toString(),
          (_) => fileCollection.get(_)?.kind ?? FileKind.UNSPECIFIED,
          (_) => `kind_${_}`,
        );

        return { key: key.toString(), [kind]: '' };
      }),

    shouldAcceptItemDrop: ({ dropPosition, key }, sourceKinds) => {
      if (dropPosition !== 'on') return false;

      const sourceCanMove =
        !sourceKinds.has(`kind_${FileKind.UNSPECIFIED}`) && !sourceKinds.has(`kind_${FileKind.HTTP_DELTA}`);
      const targetCanAccept = fileCollection.get(key.toString())?.kind === FileKind.FOLDER;

      return sourceCanMove && targetCanAccept;
    },

    onItemDrop: async ({ items, target: { dropPosition, key: targetKey } }) => {
      const [item] = items;
      if (dropPosition !== 'on' || !item || item.kind !== 'text' || items.length !== 1) return;

      const source = fileCollection.get(await item.getText('key'));
      const target = fileCollection.get(targetKey.toString());

      if (!source || !target) return;

      if (source.kind === FileKind.UNSPECIFIED) return;
      if (target.kind !== FileKind.FOLDER) return;

      fileCollection.utils.update({
        fileId: source.fileId,
        order: await getNextOrder(fileCollection),
        parentId: { kind: FileUpdate_ParentIdUnion_Kind.VALUE, value: target.fileId },
      });
    },

    onReorder: handleCollectionReorder(fileCollection),

    renderDropIndicator: () => <DropIndicatorHorizontal />,
  });

  return (
    <FileTreeContext.Provider value={{ ...context, containerRef: ref }}>
      <div className={tw`relative`} ref={ref}>
        <Tree
          aria-label='Files'
          dragAndDropHooks={dragAndDropHooks}
          items={files}
          {...(onAction && { onAction })}
          {...(onSelectionChange && { onSelectionChange })}
          {...(selectedKeys && { selectedKeys })}
          {...(selectionMode && { selectionMode })}
        >
          {(_) => <FileItem id={fileCollection.utils.getKey(_)} />}
        </Tree>
      </div>
    </FileTreeContext.Provider>
  );
};

interface FileItemProps {
  id: string;
}

const FileItem = ({ id }: FileItemProps) => {
  const fileCollection = useApiCollection(FileCollectionSchema);

  const { fileId } = useMemo(() => fileCollection.utils.parseKeyUnsafe(id), [fileCollection.utils, id]);

  const { kind } =
    useLiveQuery(
      (_) =>
        _.from({ file: fileCollection })
          .where((_) => eq(_.file.fileId, fileId))
          .select((_) => pick(_.file, 'kind'))
          .findOne(),
      [fileCollection, fileId],
    ).data ?? create(FileSchema);

  return pipe(
    Match.value(kind),
    Match.when(FileKind.FOLDER, () => <FolderFile id={id} />),
    Match.when(FileKind.HTTP, () => <HttpFile id={id} />),
    Match.when(FileKind.HTTP_DELTA, () => <HttpDeltaFile id={id} />),
    Match.when(FileKind.FLOW, () => <FlowFile id={id} />),
    Match.orElse(() => null),
  );
};

const FolderFile = ({ id }: FileItemProps) => {
  const fileCollection = useApiCollection(FileCollectionSchema);

  const { fileId: folderId } = useMemo(() => fileCollection.utils.parseKeyUnsafe(id), [fileCollection.utils, id]);

  const folderCollection = useApiCollection(FolderCollectionSchema);

  const { name } =
    useLiveQuery(
      (_) =>
        _.from({ folder: folderCollection })
          .where((_) => eq(_.folder.folderId, folderId))
          .select((_) => pick(_.folder, 'name'))
          .findOne(),
      [folderCollection, folderId],
    ).data ?? create(FolderSchema);

  const { data: files } = useLiveQuery(
    (_) =>
      _.from({ file: fileCollection })
        .where((_) => eq(_.file.parentId, folderId))
        .orderBy((_) => _.file.order)
        .select((_) => pick(_.file, 'fileId', 'order')),
    [fileCollection, folderId],
  );

  const { containerRef, showControls } = useContext(FileTreeContext);

  const { escapeRef, escapeRender } = useEscapePortal(containerRef);

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => folderCollection.utils.update({ folderId, name: _ }),
    value: name,
  });

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  return (
    <TreeItem
      id={id}
      item={(_) => <FileItem id={fileCollection.utils.getKey(_)} />}
      items={files}
      onContextMenu={onContextMenu}
      textValue={name}
    >
      {({ isExpanded }) => (
        <>
          {isExpanded ? (
            <FolderOpenedIcon className={tw`size-4 text-slate-500`} />
          ) : (
            <FiFolder className={tw`size-4 text-slate-500`} />
          )}

          <Text className={twJoin(tw`flex-1 truncate`, isEditing && tw`opacity-0`)} ref={escapeRef}>
            {name}
          </Text>

          {isEditing &&
            escapeRender(
              <TextInputField
                aria-label='Folder name'
                className={tw`w-full`}
                inputClassName={tw`-my-1 py-1`}
                {...textFieldProps}
              />,
            )}

          {showControls && (
            <MenuTrigger {...menuTriggerProps}>
              <Button className={tw`p-0.5`} variant='ghost'>
                <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
              </Button>

              <Menu {...menuProps}>
                <SubmenuTrigger>
                  <MenuItem>New</MenuItem>

                  <FileCreateMenu parentFolderId={folderId} />
                </SubmenuTrigger>

                <MenuItem onAction={() => void edit()}>Rename</MenuItem>

                <MenuItem
                  onAction={() => pipe(fileCollection.utils.parseKeyUnsafe(id), (_) => fileCollection.utils.delete(_))}
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

const HttpFile = ({ id }: FileItemProps) => {
  const matchRoute = useMatchRoute();
  const navigate = useNavigate();

  const { workspaceId } = workspaceRouteApi.useLoaderData();

  const fileCollection = useApiCollection(FileCollectionSchema);

  const { fileId: httpId } = useMemo(() => fileCollection.utils.parseKeyUnsafe(id), [fileCollection.utils, id]);

  const httpCollection = useApiCollection(HttpCollectionSchema);

  const { method, name } =
    useLiveQuery(
      (_) =>
        _.from({ http: httpCollection })
          .where((_) => eq(_.http.httpId, httpId))
          .select((_) => pick(_.http, 'name', 'method'))
          .findOne(),
      [httpCollection, httpId],
    ).data ?? create(HttpSchema);

  const deltaCollection = useApiCollection(HttpDeltaCollectionSchema);

  const { data: files } = useLiveQuery(
    (_) =>
      _.from({ file: fileCollection })
        .where((_) => eq(_.file.parentId, httpId))
        .orderBy((_) => _.file.order)
        .select((_) => pick(_.file, 'fileId', 'order')),
    [fileCollection, httpId],
  );

  const modal = useProgrammaticModal();

  const duplicateMutation = useConnectMutation(HttpService.method.httpDuplicate);
  const exportMutation = useConnectMutation(ExportService.method.export);
  const exportCurlMutation = useConnectMutation(ExportService.method.exportCurl);

  const { containerRef, navigate: toNavigate = false, showControls } = useContext(FileTreeContext);

  const { escapeRef, escapeRender } = useEscapePortal(containerRef);

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => httpCollection.utils.update({ httpId, name: _ }),
    value: name,
  });

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const route = {
    from: workspaceRouteApi.id,
    params: { httpIdCan: Ulid.construct(httpId).toCanonical() },
    to: httpRouteApi.id,
  } satisfies ToOptions;

  const content = (
    <>
      {modal.children && <Modal {...modal} size='sm' />}

      <MethodBadge method={method} />

      <Text className={twJoin(tw`flex-1 truncate`, isEditing && tw`opacity-0`)} ref={escapeRef}>
        {name}
      </Text>

      {isEditing &&
        escapeRender(
          <TextInputField
            aria-label='HTTP request name'
            className={tw`w-full`}
            inputClassName={tw`-my-1 py-1`}
            {...textFieldProps}
          />,
        )}

      {showControls && (
        <MenuTrigger {...menuTriggerProps}>
          <Button className={tw`p-0.5`} variant='ghost'>
            <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
          </Button>

          <Menu {...menuProps}>
            <MenuItem
              onAction={async () => {
                const deltaHttpId = Ulid.generate().bytes;
                deltaCollection.utils.insert({ deltaHttpId, httpId });
                fileCollection.utils.insert({
                  fileId: deltaHttpId,
                  kind: FileKind.HTTP_DELTA,
                  order: await getNextOrder(fileCollection),
                  parentId: httpId,
                  workspaceId,
                });
                await navigate({
                  from: workspaceRouteApi.id,
                  params: {
                    deltaHttpIdCan: Ulid.construct(deltaHttpId).toCanonical(),
                    httpIdCan: Ulid.construct(httpId).toCanonical(),
                  },
                  to: httpDeltaRouteApi.id,
                });
              }}
            >
              New delta
            </MenuItem>

            <MenuItem onAction={() => void edit()}>Rename</MenuItem>

            <MenuItem onAction={() => duplicateMutation.mutateAsync({ httpId })}>Duplicate</MenuItem>

            <SubmenuTrigger>
              <MenuItem>Export</MenuItem>

              <Menu>
                <MenuItem
                  onAction={async () => {
                    const { data, name } = await exportMutation.mutateAsync({ fileIds: [httpId], workspaceId });
                    saveFile({ blobParts: [data], name });
                  }}
                >
                  YAML (DevTools)
                </MenuItem>

                <MenuItem
                  onAction={async () => {
                    const { data } = await exportCurlMutation.mutateAsync({ httpIds: [httpId], workspaceId });
                    modal.onOpenChange(
                      true,
                      <Dialog className={tw`flex h-full flex-col gap-4 p-6`}>
                        {({ close }) => (
                          <>
                            <div className={tw`flex items-center justify-between`}>
                              <Heading
                                className={tw`text-xl leading-6 font-semibold tracking-tighter text-slate-800`}
                                slot='title'
                              >
                                cURL export
                              </Heading>

                              <Button className={tw`p-1`} onPress={() => void close()} variant='ghost'>
                                <FiX className={tw`size-5 text-slate-500`} />
                              </Button>
                            </div>

                            <CodeMirror className={tw`flex-1`} height='100%' readOnly value={data} />
                          </>
                        )}
                      </Dialog>,
                    );
                  }}
                >
                  cURL
                </MenuItem>
              </Menu>
            </SubmenuTrigger>

            <MenuItem
              onAction={() => pipe(fileCollection.utils.parseKeyUnsafe(id), (_) => fileCollection.utils.delete(_))}
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
    id,
    item: (_) => <FileItem id={fileCollection.utils.getKey(_)} />,
    items: files,
    onContextMenu,
    textValue: name,
  } satisfies TreeItemProps<(typeof files)[number]>;

  return toNavigate ? <TreeItemLink {...props} {...route} /> : <TreeItem {...props} />;
};

const HttpDeltaFile = ({ id }: FileItemProps) => {
  const matchRoute = useMatchRoute();

  const { workspaceId } = workspaceRouteApi.useLoaderData();

  const fileCollection = useApiCollection(FileCollectionSchema);

  const { fileId: deltaHttpId } = useMemo(() => fileCollection.utils.parseKeyUnsafe(id), [fileCollection.utils, id]);

  const deltaCollection = useApiCollection(HttpDeltaCollectionSchema);

  const { httpId } =
    useLiveQuery(
      (_) =>
        _.from({ item: deltaCollection })
          .where((_) => eq(_.item.deltaHttpId, deltaHttpId))
          .select((_) => pick(_.item, 'httpId'))
          .findOne(),
      [deltaCollection, deltaHttpId],
    ).data ?? create(HttpDeltaSchema);

  const deltaOptions = {
    deltaId: deltaHttpId,
    deltaSchema: HttpDeltaCollectionSchema,
    originId: httpId,
    originSchema: HttpCollectionSchema,
  } as const;

  const [name, setName] = useDeltaState({ ...deltaOptions, valueKey: 'name' });
  const [method] = useDeltaState({ ...deltaOptions, valueKey: 'method' });

  const modal = useProgrammaticModal();

  const exportMutation = useConnectMutation(ExportService.method.export);
  const exportCurlMutation = useConnectMutation(ExportService.method.exportCurl);

  const { containerRef, navigate: toNavigate = false, showControls } = useContext(FileTreeContext);

  const { escapeRef, escapeRender } = useEscapePortal(containerRef);

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => {
      if (_ === name) return;
      setName(_);
    },
    value: name ?? '',
  });

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const route = {
    from: workspaceRouteApi.id,
    params: {
      deltaHttpIdCan: Ulid.construct(deltaHttpId).toCanonical(),
      httpIdCan: Ulid.construct(httpId).toCanonical(),
    },
    to: httpDeltaRouteApi.id,
  } satisfies ToOptions;

  const content = (
    <>
      {modal.children && <Modal {...modal} size='sm' />}

      <MethodBadge method={method ?? HttpMethod.UNSPECIFIED} />

      <Text className={twJoin(tw`flex-1 truncate`, isEditing && tw`opacity-0`)} ref={escapeRef}>
        {name}
      </Text>

      {isEditing &&
        escapeRender(
          <TextInputField
            aria-label='HTTP request name'
            className={tw`w-full`}
            inputClassName={tw`-my-1 py-1`}
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

            <SubmenuTrigger>
              <MenuItem>Export</MenuItem>

              <Menu>
                <MenuItem
                  onAction={async () => {
                    const { data, name } = await exportMutation.mutateAsync({ fileIds: [deltaHttpId], workspaceId });
                    saveFile({ blobParts: [data], name });
                  }}
                >
                  YAML (DevTools)
                </MenuItem>

                <MenuItem
                  onAction={async () => {
                    const { data } = await exportCurlMutation.mutateAsync({ httpIds: [deltaHttpId], workspaceId });
                    modal.onOpenChange(
                      true,
                      <Dialog className={tw`flex h-full flex-col gap-4 p-6`}>
                        {({ close }) => (
                          <>
                            <div className={tw`flex items-center justify-between`}>
                              <Heading
                                className={tw`text-xl leading-6 font-semibold tracking-tighter text-slate-800`}
                                slot='title'
                              >
                                cURL export
                              </Heading>

                              <Button className={tw`p-1`} onPress={() => void close()} variant='ghost'>
                                <FiX className={tw`size-5 text-slate-500`} />
                              </Button>
                            </div>

                            <CodeMirror className={tw`flex-1`} height='100%' readOnly value={data} />
                          </>
                        )}
                      </Dialog>,
                    );
                  }}
                >
                  cURL
                </MenuItem>
              </Menu>
            </SubmenuTrigger>

            <MenuItem onAction={() => void fileCollection.utils.delete({ fileId: deltaHttpId })} variant='danger'>
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
    id,
    onContextMenu,
    textValue: name ?? '',
  } satisfies TreeItemProps<object>;

  return toNavigate ? <TreeItemLink {...props} {...route} /> : <TreeItem {...props} />;
};

const FlowFile = ({ id }: FileItemProps) => {
  const matchRoute = useMatchRoute();

  const { workspaceId } = workspaceRouteApi.useLoaderData();

  const fileCollection = useApiCollection(FileCollectionSchema);

  const { fileId: flowId } = useMemo(() => fileCollection.utils.parseKeyUnsafe(id), [fileCollection.utils, id]);

  const flowCollection = useApiCollection(FlowCollectionSchema);

  const { name } =
    useLiveQuery(
      (_) =>
        _.from({ flow: flowCollection })
          .where((_) => eq(_.flow.flowId, flowId))
          .select((_) => pick(_.flow, 'name'))
          .findOne(),
      [flowCollection, flowId],
    ).data ?? create(FlowSchema);

  const duplicateMutation = useConnectMutation(FlowService.method.flowDuplicate);
  const exportMutation = useConnectMutation(ExportService.method.export);

  const { containerRef, navigate: toNavigate = false, showControls } = useContext(FileTreeContext);

  const { escapeRef, escapeRender } = useEscapePortal(containerRef);

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => flowCollection.utils.update({ flowId, name: _ }),
    value: name,
  });

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const route = {
    from: workspaceRouteApi.id,
    params: { flowIdCan: Ulid.construct(flowId).toCanonical() },
    to: flowLayoutRouteApi.id,
  } satisfies ToOptions;

  const content = (
    <>
      <FlowsIcon className={tw`size-4 text-slate-500`} />

      <Text className={twJoin(tw`flex-1 truncate`, isEditing && tw`opacity-0`)} ref={escapeRef}>
        {name}
      </Text>

      {isEditing &&
        escapeRender(
          <TextInputField
            aria-label='Flow name'
            className={tw`w-full`}
            inputClassName={tw`-my-1 py-1`}
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

            <MenuItem onAction={() => duplicateMutation.mutateAsync({ flowId })}>Duplicate</MenuItem>

            <MenuItem
              onAction={async () => {
                const { data, name } = await exportMutation.mutateAsync({ fileIds: [flowId], workspaceId });
                saveFile({ blobParts: [data], name });
              }}
            >
              Export YAML (DevTools)
            </MenuItem>

            <MenuItem
              onAction={() => pipe(fileCollection.utils.parseKeyUnsafe(id), (_) => fileCollection.utils.delete(_))}
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
    id,
    onContextMenu,
    textValue: name,
  } satisfies TreeItemProps<object>;

  return toNavigate ? <TreeItemLink {...props} {...route} /> : <TreeItem {...props} />;
};
