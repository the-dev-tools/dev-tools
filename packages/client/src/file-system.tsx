import { enumToJson } from '@bufbuild/protobuf';
import { eq, isUndefined, useLiveQuery } from '@tanstack/react-db';
import { Match, Option, pipe } from 'effect';
import { Ulid } from 'id128';
import { createContext, RefObject, useContext, useMemo, useRef } from 'react';
import { MenuTrigger, SubmenuTrigger, Text, Tree, useDragAndDrop } from 'react-aria-components';
import { FiFolder, FiMoreHorizontal } from 'react-icons/fi';
import { twJoin } from 'tailwind-merge';
import {
  File,
  FileKind,
  FileKindJson,
  FileKindSchema,
  FileUpdate_ParentFolderIdUnion_Kind,
} from '@the-dev-tools/spec/api/file_system/v1/file_system_pb';
import { HttpMethod } from '@the-dev-tools/spec/api/http/v1/http_pb';
import { FileCollectionSchema, FolderCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/file_system';
import { FlowCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { HttpCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { Button } from '@the-dev-tools/ui/button';
import { FlowsIcon, FolderOpenedIcon } from '@the-dev-tools/ui/icons';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { MethodBadge } from '@the-dev-tools/ui/method-badge';
import { DropIndicatorHorizontal } from '@the-dev-tools/ui/reorder';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { TreeItem } from '@the-dev-tools/ui/tree';
import { useEscapePortal } from '@the-dev-tools/ui/utils';
import { useApiCollection } from '~/api-new';
import { workspaceRouteApi } from '~/routes';
import { getNextOrder, handleCollectionReorder } from '~/utils/order';
import { pick } from '~/utils/tanstack-db';

interface FileCreateMenuProps {
  parentFolderId?: Uint8Array;
}

export const FileCreateMenu = ({ parentFolderId }: FileCreateMenuProps) => {
  const { workspaceId } = workspaceRouteApi.useLoaderData();

  const fileCollection = useApiCollection(FileCollectionSchema);
  const folderCollection = useApiCollection(FolderCollectionSchema);
  const httpCollection = useApiCollection(HttpCollectionSchema);
  const flowCollection = useApiCollection(FlowCollectionSchema);

  const insertFile = async (props: Pick<File, 'fileId' | 'kind'>) =>
    fileCollection.utils.insert({
      ...props,
      ...(parentFolderId && { parentFolderId }),
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
        onAction={() => {
          const httpId = Ulid.generate().bytes;
          httpCollection.utils.insert({ httpId, method: HttpMethod.GET, name: 'New HTTP request' });
          void insertFile({ fileId: httpId, kind: FileKind.HTTP });
        }}
      >
        HTTP request
      </MenuItem>

      <MenuItem
        onAction={() => {
          const flowId = Ulid.generate().bytes;
          flowCollection.utils.insert({ flowId, name: 'New flow' });
          void insertFile({ fileId: flowId, kind: FileKind.FLOW });
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

interface FileTreeProps extends Omit<FileTreeContext, 'containerRef'> {}

export const FileTree = ({ ...context }: FileTreeProps) => {
  const { workspaceId } = workspaceRouteApi.useLoaderData();

  const fileCollection = useApiCollection(FileCollectionSchema);

  const { data: files } = useLiveQuery(
    (_) =>
      _.from({ file: fileCollection })
        .where((_) => eq(_.file.workspaceId, workspaceId))
        .where((_) => isUndefined(_.file.parentFolderId))
        .orderBy((_) => _.file.order)
        .select((_) => pick(_.file, 'fileId', 'order')),
    [fileCollection, workspaceId],
  );

  const ref = useRef<HTMLDivElement>(null);

  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) =>
      [...keys].map((key) => {
        const kind = fileCollection.get(key.toString())?.kind ?? FileKind.UNSPECIFIED;
        return { key: key.toString(), [kind]: enumToJson(FileKindSchema, kind) };
      }),

    shouldAcceptItemDrop: ({ dropPosition, key }, sourceKinds) => {
      if (dropPosition !== 'on') return false;

      const sourceCanMove = !sourceKinds.has('FILE_KIND_UNSPECIFIED' satisfies FileKindJson);
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
        parentFolderId: { bytes: target.fileId, kind: FileUpdate_ParentFolderIdUnion_Kind.BYTES },
      });
    },

    onReorder: handleCollectionReorder(fileCollection),

    renderDropIndicator: () => <DropIndicatorHorizontal />,
  });

  return (
    <FileTreeContext.Provider value={{ ...context, containerRef: ref }}>
      <div className={tw`relative`} ref={ref}>
        <Tree aria-label='Files' dragAndDropHooks={dragAndDropHooks} items={files}>
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

  const { kind } = pipe(
    useLiveQuery(
      (_) =>
        _.from({ file: fileCollection })
          .where((_) => eq(_.file.fileId, fileId))
          .select((_) => pick(_.file, 'kind'))
          .findOne(),
      [fileCollection, fileId],
    ),
    (_) => Option.fromNullable(_.data),
    Option.getOrThrow,
  );

  return pipe(
    Match.value(kind),
    Match.when(FileKind.FOLDER, () => <FolderFile id={id} />),
    Match.when(FileKind.HTTP, () => <HttpFile id={id} />),
    Match.when(FileKind.FLOW, () => <FlowFile id={id} />),
    Match.orElseAbsurd,
  );
};

const FolderFile = ({ id }: FileItemProps) => {
  const fileCollection = useApiCollection(FileCollectionSchema);

  const { fileId: folderId } = useMemo(() => fileCollection.utils.parseKeyUnsafe(id), [fileCollection.utils, id]);

  const folderCollection = useApiCollection(FolderCollectionSchema);

  const { name } = pipe(
    useLiveQuery(
      (_) =>
        _.from({ folder: folderCollection })
          .where((_) => eq(_.folder.folderId, folderId))
          .select((_) => pick(_.folder, 'name'))
          .findOne(),
      [folderCollection, folderId],
    ),
    (_) => Option.fromNullable(_.data),
    Option.getOrThrow,
  );

  const { data: files } = useLiveQuery(
    (_) =>
      _.from({ file: fileCollection })
        .where((_) => eq(_.file.parentFolderId, folderId))
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
  const fileCollection = useApiCollection(FileCollectionSchema);

  const { fileId: httpId } = useMemo(() => fileCollection.utils.parseKeyUnsafe(id), [fileCollection.utils, id]);

  const httpCollection = useApiCollection(HttpCollectionSchema);

  const { method, name } = pipe(
    useLiveQuery(
      (_) =>
        _.from({ http: httpCollection })
          .where((_) => eq(_.http.httpId, httpId))
          .select((_) => pick(_.http, 'name', 'method'))
          .findOne(),
      [httpCollection, httpId],
    ),
    (_) => Option.fromNullable(_.data),
    Option.getOrThrow,
  );

  const { containerRef, showControls } = useContext(FileTreeContext);

  const { escapeRef, escapeRender } = useEscapePortal(containerRef);

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => httpCollection.utils.update({ httpId, name: _ }),
    value: name,
  });

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  return (
    <TreeItem id={id} onContextMenu={onContextMenu} textValue={name}>
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
    </TreeItem>
  );
};

const FlowFile = ({ id }: FileItemProps) => {
  const fileCollection = useApiCollection(FileCollectionSchema);

  const { fileId: flowId } = useMemo(() => fileCollection.utils.parseKeyUnsafe(id), [fileCollection.utils, id]);

  const flowCollection = useApiCollection(FlowCollectionSchema);

  const { name } = pipe(
    useLiveQuery(
      (_) =>
        _.from({ flow: flowCollection })
          .where((_) => eq(_.flow.flowId, flowId))
          .select((_) => pick(_.flow, 'name'))
          .findOne(),
      [flowCollection, flowId],
    ),
    (_) => Option.fromNullable(_.data),
    Option.getOrThrow,
  );

  const { containerRef, showControls } = useContext(FileTreeContext);

  const { escapeRef, escapeRender } = useEscapePortal(containerRef);

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => flowCollection.utils.update({ flowId, name: _ }),
    value: name,
  });

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  return (
    <TreeItem id={id} onContextMenu={onContextMenu} textValue={name}>
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

            <MenuItem
              onAction={() => pipe(fileCollection.utils.parseKeyUnsafe(id), (_) => fileCollection.utils.delete(_))}
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
