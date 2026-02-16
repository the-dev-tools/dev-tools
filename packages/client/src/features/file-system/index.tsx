import { create } from '@bufbuild/protobuf';
import { and, eq, isUndefined, or, useLiveQuery } from '@tanstack/react-db';
import { linkOptions, ToOptions, useMatchRoute, useNavigate, useRouter } from '@tanstack/react-router';
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
import { RiAnthropicFill, RiGeminiFill, RiOpenaiFill } from 'react-icons/ri';
import { TbGauge } from 'react-icons/tb';
import { twJoin } from 'tailwind-merge';
import { Credential, CredentialKind, CredentialSchema } from '@the-dev-tools/spec/buf/api/credential/v1/credential_pb';
import { ExportService } from '@the-dev-tools/spec/buf/api/export/v1/export_pb';
import {
  File,
  FileKind,
  FileSchema,
  FileUpdate_ParentIdUnion_Kind,
  FolderSchema,
} from '@the-dev-tools/spec/buf/api/file_system/v1/file_system_pb';
import { FlowSchema, FlowService } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { GraphQLSchema as GraphQLItemSchema } from '@the-dev-tools/spec/buf/api/graph_q_l/v1/graph_q_l_pb';
import { HttpDeltaSchema, HttpMethod, HttpSchema, HttpService } from '@the-dev-tools/spec/buf/api/http/v1/http_pb';
import {
  CredentialAnthropicCollectionSchema,
  CredentialCollectionSchema,
  CredentialGeminiCollectionSchema,
  CredentialOpenAiCollectionSchema,
} from '@the-dev-tools/spec/tanstack-db/v1/api/credential';
import { FileCollectionSchema, FolderCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/file_system';
import { FlowCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/flow';
import { GraphQLCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/graph_q_l';
import { HttpCollectionSchema, HttpDeltaCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/http';
import { Button } from '@the-dev-tools/ui/button';
import { FlowsIcon, FolderOpenedIcon } from '@the-dev-tools/ui/icons';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { MethodBadge } from '@the-dev-tools/ui/method-badge';
import { Modal, useProgrammaticModal } from '@the-dev-tools/ui/modal';
import { DropIndicatorHorizontal } from '@the-dev-tools/ui/reorder';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { useTheme } from '@the-dev-tools/ui/theme';
import { TreeItem, TreeItemProps, TreeItemRouteLink } from '@the-dev-tools/ui/tree';
import { saveFile, useEscapePortal } from '@the-dev-tools/ui/utils';
import { useDeltaState } from '~/features/delta';
import { useApiCollection, useConnectMutation } from '~/shared/api';
import { eqStruct, getNextOrder, handleCollectionReorder, pick } from '~/shared/lib';
import { routes } from '~/shared/routes';

const useInsertFile = (parentFolderId?: Uint8Array) => {
  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();

  const fileCollection = useApiCollection(FileCollectionSchema);

  return async (props: Pick<File, 'fileId' | 'kind'>) =>
    fileCollection.utils.insert({
      ...props,
      ...(parentFolderId && { parentId: parentFolderId }),
      order: await getNextOrder(fileCollection),
      workspaceId,
    });
};

interface FileCreateMenuProps {
  navigate?: boolean;
  parentFolderId?: Uint8Array;
}

export const FileCreateMenu = ({ parentFolderId, ...props }: FileCreateMenuProps) => {
  const router = useRouter();
  const navigate = useNavigate();

  const fileTreeContext = useContext(FileTreeContext);
  const toNavigate = props.navigate ?? fileTreeContext.navigate ?? false;

  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();

  const folderCollection = useApiCollection(FolderCollectionSchema);
  const graphqlCollection = useApiCollection(GraphQLCollectionSchema);
  const httpCollection = useApiCollection(HttpCollectionSchema);
  const flowCollection = useApiCollection(FlowCollectionSchema);

  const insertFile = useInsertFile(parentFolderId);

  return (
    <Menu>
      <MenuItem
        onAction={() => {
          const folderId = Ulid.generate().bytes;
          folderCollection.utils.insert({ folderId, name: 'New folder' });
        }}
      >
        Folder
      </MenuItem>

      <MenuItem
        onAction={async () => {
          const httpUlid = Ulid.generate();
          httpCollection.utils.insert({ httpId: httpUlid.bytes, method: HttpMethod.GET, name: 'New HTTP request' });
          await insertFile({ fileId: httpUlid.bytes, kind: FileKind.HTTP });
          if (toNavigate)
            await navigate({
              from: router.routesById[routes.dashboard.workspace.route.id].fullPath,
              params: { httpIdCan: httpUlid.toCanonical() },
              to: router.routesById[routes.dashboard.workspace.http.route.id].fullPath,
            });
        }}
      >
        HTTP request
      </MenuItem>

      <MenuItem
        onAction={async () => {
          const graphqlUlid = Ulid.generate();
          graphqlCollection.utils.insert({ graphqlId: graphqlUlid.bytes, name: 'New GraphQL request' });
          await insertFile({ fileId: graphqlUlid.bytes, kind: FileKind.GRAPH_Q_L });
          if (toNavigate)
            await navigate({
              from: router.routesById[routes.dashboard.workspace.route.id].fullPath,
              params: { graphqlIdCan: graphqlUlid.toCanonical() },
              to: router.routesById[routes.dashboard.workspace.graphql.route.id].fullPath,
            });
        }}
      >
        GraphQL request
      </MenuItem>

      <MenuItem
        onAction={async () => {
          const flowUlid = Ulid.generate();
          flowCollection.utils.insert({ flowId: flowUlid.bytes, name: 'New flow', workspaceId });
          await insertFile({ fileId: flowUlid.bytes, kind: FileKind.FLOW });

          if (toNavigate)
            await navigate({
              from: router.routesById[routes.dashboard.workspace.route.id].fullPath,
              params: { flowIdCan: flowUlid.toCanonical() },
              to: router.routesById[routes.dashboard.workspace.flow.route.id].fullPath,
            });
        }}
      >
        Flow
      </MenuItem>

      <CreateCredentialSubmenu navigate={toNavigate} {...(parentFolderId && { parentFolderId })} />
    </Menu>
  );
};

const CreateCredentialSubmenu = ({ navigate: toNavigate, parentFolderId }: FileCreateMenuProps) => {
  const router = useRouter();
  const navigate = useNavigate();

  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();

  const credentialCollection = useApiCollection(CredentialCollectionSchema);
  const credentialOpenAiCollection = useApiCollection(CredentialOpenAiCollectionSchema);
  const credentialGeminiCollection = useApiCollection(CredentialGeminiCollectionSchema);
  const credentialAnthropicCollection = useApiCollection(CredentialAnthropicCollectionSchema);

  const insertFile = useInsertFile(parentFolderId);

  const insertBase = async ({ kind, name }: Pick<Credential, 'kind' | 'name'>) => {
    const credentialId = Ulid.generate().bytes;
    credentialCollection.utils.insert({ credentialId, kind, name: `${name} credential`, workspaceId });
    await insertFile({ fileId: credentialId, kind: FileKind.CREDENTIAL });
    return credentialId;
  };

  const open = async (credentialId: Uint8Array) => {
    if (!toNavigate) return;

    await navigate({
      from: router.routesById[routes.dashboard.workspace.route.id].fullPath,
      params: { credentialIdCan: Ulid.construct(credentialId).toCanonical() },
      to: router.routesById[routes.dashboard.workspace.credential.id].fullPath,
    });
  };

  return (
    <SubmenuTrigger>
      <MenuItem>Credential</MenuItem>

      <Menu>
        <MenuItem
          onAction={async () => {
            const credentialId = await insertBase({ kind: CredentialKind.OPEN_AI, name: 'OpenAI' });
            credentialOpenAiCollection.utils.insert({ credentialId });
            await open(credentialId);
          }}
        >
          OpenAI
        </MenuItem>

        <MenuItem
          onAction={async () => {
            const credentialId = await insertBase({ kind: CredentialKind.GEMINI, name: 'Gemini' });
            credentialGeminiCollection.utils.insert({ credentialId });
            await open(credentialId);
          }}
        >
          Gemini
        </MenuItem>

        <MenuItem
          onAction={async () => {
            const credentialId = await insertBase({ kind: CredentialKind.ANTHROPIC, name: 'Anthropic' });
            credentialAnthropicCollection.utils.insert({ credentialId });
            await open(credentialId);
          }}
        >
          Anthropic
        </MenuItem>
      </Menu>
    </SubmenuTrigger>
  );
};

interface FileTreeContext {
  containerRef: RefObject<HTMLDivElement | null>;
  kind?: FileKind;
  navigate?: boolean;
  showControls?: boolean;
}

const FileTreeContext = createContext({} as FileTreeContext);

interface FileTreeProps
  extends
    Omit<FileTreeContext, 'containerRef'>,
    Pick<TreeProps<object>, 'onAction' | 'onSelectionChange' | 'selectedKeys' | 'selectionMode'> {}

export const FileTree = ({ onAction, onSelectionChange, selectedKeys, selectionMode, ...context }: FileTreeProps) => {
  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();
  const { kind } = context;

  const fileCollection = useApiCollection(FileCollectionSchema);

  const { data: files } = useLiveQuery(
    (_) => {
      let query = _.from({ file: fileCollection }).where((_) =>
        and(eq(_.file.workspaceId, workspaceId), isUndefined(_.file.parentId)),
      );

      if (kind) query = query.where((_) => or(eq(_.file.kind, kind), eq(_.file.kind, FileKind.FOLDER)));

      return query.orderBy((_) => _.file.order).select((_) => pick(_.file, 'fileId', 'order'));
    },
    [fileCollection, kind, workspaceId],
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
      if (dropPosition !== 'on' || item?.kind !== 'text' || items.length !== 1) return;

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
    Match.when(FileKind.GRAPH_Q_L, () => <GraphQLFile id={id} />),
    Match.when(FileKind.CREDENTIAL, () => <CredentialFile id={id} />),
    Match.orElse(() => null),
  );
};

const FolderFile = ({ id }: FileItemProps) => {
  const fileCollection = useApiCollection(FileCollectionSchema);

  const { containerRef, kind, showControls } = useContext(FileTreeContext);

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
    (_) => {
      let query = _.from({ file: fileCollection }).where((_) => eq(_.file.parentId, folderId));

      if (kind) query = query.where((_) => or(eq(_.file.kind, kind), eq(_.file.kind, FileKind.FOLDER)));

      return query.orderBy((_) => _.file.order).select((_) => pick(_.file, 'fileId', 'order'));
    },
    [fileCollection, folderId, kind],
  );

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
          {name === 'Credentials' ? (
            <TbGauge className={tw`size-4 text-on-neutral-low`} />
          ) : isExpanded ? (
            <FolderOpenedIcon className={tw`size-4 text-on-neutral-low`} />
          ) : (
            <FiFolder className={tw`size-4 text-on-neutral-low`} />
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
                <FiMoreHorizontal className={tw`size-4 text-on-neutral-low`} />
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
  const router = useRouter();
  const navigate = useNavigate();

  const { theme } = useTheme();

  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();

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
    from: router.routesById[routes.dashboard.workspace.route.id].fullPath,
    params: { httpIdCan: Ulid.construct(httpId).toCanonical() },
    to: router.routesById[routes.dashboard.workspace.http.route.id].fullPath,
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
            <FiMoreHorizontal className={tw`size-4 text-on-neutral-low`} />
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
                if (toNavigate)
                  await navigate({
                    from: router.routesById[routes.dashboard.workspace.route.id].fullPath,
                    params: {
                      deltaHttpIdCan: Ulid.construct(deltaHttpId).toCanonical(),
                      httpIdCan: Ulid.construct(httpId).toCanonical(),
                    },
                    to: router.routesById[routes.dashboard.workspace.http.delta.id].fullPath,
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
                                className={tw`text-xl leading-6 font-semibold tracking-tighter text-on-neutral`}
                                slot='title'
                              >
                                cURL export
                              </Heading>

                              <Button className={tw`p-1`} onPress={() => void close()} variant='ghost'>
                                <FiX className={tw`size-5 text-on-neutral-low`} />
                              </Button>
                            </div>

                            <CodeMirror className={tw`flex-1`} height='100%' readOnly theme={theme} value={data} />
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
    className: toNavigate && matchRoute(route) !== false ? tw`bg-neutral` : '',
    id,
    item: (_) => <FileItem id={fileCollection.utils.getKey(_)} />,
    items: files,
    onContextMenu,
    textValue: name,
  } satisfies TreeItemProps<(typeof files)[number]>;

  return toNavigate ? <TreeItemRouteLink {...props} {...route} /> : <TreeItem {...props} />;
};

const HttpDeltaFile = ({ id }: FileItemProps) => {
  const router = useRouter();
  const matchRoute = useMatchRoute();

  const { theme } = useTheme();

  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();

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
    from: router.routesById[routes.dashboard.workspace.route.id].fullPath,
    params: {
      deltaHttpIdCan: Ulid.construct(deltaHttpId).toCanonical(),
      httpIdCan: Ulid.construct(httpId).toCanonical(),
    },
    to: router.routesById[routes.dashboard.workspace.http.delta.id].fullPath,
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
            <FiMoreHorizontal className={tw`size-4 text-on-neutral-low`} />
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
                                className={tw`text-xl leading-6 font-semibold tracking-tighter text-on-neutral`}
                                slot='title'
                              >
                                cURL export
                              </Heading>

                              <Button className={tw`p-1`} onPress={() => void close()} variant='ghost'>
                                <FiX className={tw`size-5 text-on-neutral-low`} />
                              </Button>
                            </div>

                            <CodeMirror className={tw`flex-1`} height='100%' readOnly theme={theme} value={data} />
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
    className: toNavigate && matchRoute(route) !== false ? tw`bg-neutral` : '',
    id,
    onContextMenu,
    textValue: name ?? '',
  } satisfies TreeItemProps<object>;

  return toNavigate ? <TreeItemRouteLink {...props} {...route} /> : <TreeItem {...props} />;
};

const FlowFile = ({ id }: FileItemProps) => {
  const router = useRouter();
  const matchRoute = useMatchRoute();

  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();

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
    from: router.routesById[routes.dashboard.workspace.route.id].fullPath,
    params: { flowIdCan: Ulid.construct(flowId).toCanonical() },
    to: router.routesById[routes.dashboard.workspace.flow.route.id].fullPath,
  } satisfies ToOptions;

  const content = (
    <>
      <FlowsIcon className={tw`size-4 text-on-neutral-low`} />

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
            <FiMoreHorizontal className={tw`size-4 text-on-neutral-low`} />
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
    className: toNavigate && matchRoute(route) !== false ? tw`bg-neutral` : '',
    id,
    onContextMenu,
    textValue: name,
  } satisfies TreeItemProps<object>;

  return toNavigate ? <TreeItemRouteLink {...props} {...route} /> : <TreeItem {...props} />;
};

const GraphQLFile = ({ id }: FileItemProps) => {
  const router = useRouter();
  const matchRoute = useMatchRoute();

  const fileCollection = useApiCollection(FileCollectionSchema);

  const { fileId: graphqlId } = useMemo(() => fileCollection.utils.parseKeyUnsafe(id), [fileCollection.utils, id]);

  const graphqlCollection = useApiCollection(GraphQLCollectionSchema);

  const { name } =
    useLiveQuery(
      (_) =>
        _.from({ item: graphqlCollection })
          .where((_) => eq(_.item.graphqlId, graphqlId))
          .select((_) => pick(_.item, 'name'))
          .findOne(),
      [graphqlCollection, graphqlId],
    ).data ?? create(GraphQLItemSchema);

  const { containerRef, navigate: toNavigate = false, showControls } = useContext(FileTreeContext);

  const { escapeRef, escapeRender } = useEscapePortal(containerRef);

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => graphqlCollection.utils.update({ graphqlId, name: _ }),
    value: name,
  });

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const route = {
    from: router.routesById[routes.dashboard.workspace.route.id].fullPath,
    params: { graphqlIdCan: Ulid.construct(graphqlId).toCanonical() },
    to: router.routesById[routes.dashboard.workspace.graphql.route.id].fullPath,
  } satisfies ToOptions;

  const content = (
    <>
      <span className={tw`rounded bg-pink-100 px-1.5 py-0.5 text-[10px] font-semibold text-pink-700`}>GQL</span>

      <Text className={twJoin(tw`flex-1 truncate`, isEditing && tw`opacity-0`)} ref={escapeRef}>
        {name}
      </Text>

      {isEditing &&
        escapeRender(
          <TextInputField
            aria-label='GraphQL request name'
            className={tw`w-full`}
            inputClassName={tw`-my-1 py-1`}
            {...textFieldProps}
          />,
        )}

      {showControls && (
        <MenuTrigger {...menuTriggerProps}>
          <Button className={tw`p-0.5`} variant='ghost'>
            <FiMoreHorizontal className={tw`size-4 text-on-neutral-low`} />
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
    </>
  );

  const props = {
    children: content,
    className: toNavigate && matchRoute(route) !== false ? tw`bg-neutral` : '',
    id,
    onContextMenu,
    textValue: name,
  } satisfies TreeItemProps<object>;

  return toNavigate ? <TreeItemRouteLink {...props} {...route} /> : <TreeItem {...props} />;
};

const CredentialFile = ({ id }: FileItemProps) => {
  const router = useRouter();
  const matchRoute = useMatchRoute();

  const fileCollection = useApiCollection(FileCollectionSchema);

  const { fileId: credentialId } = useMemo(() => fileCollection.utils.parseKeyUnsafe(id), [fileCollection.utils, id]);

  const credentialCollection = useApiCollection(CredentialCollectionSchema);

  const { kind, name } =
    useLiveQuery(
      (_) =>
        _.from({ item: credentialCollection })
          .where(eqStruct({ credentialId }))
          .select((_) => pick(_.item, 'name', 'kind'))
          .findOne(),
      [credentialCollection, credentialId],
    ).data ?? create(CredentialSchema);

  const { containerRef, navigate: toNavigate = false, showControls } = useContext(FileTreeContext);

  const { escapeRef, escapeRender } = useEscapePortal(containerRef);

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => credentialCollection.utils.update({ credentialId, name: _ }),
    value: name,
  });

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const route = linkOptions({
    from: router.routesById[routes.dashboard.workspace.route.id].fullPath,
    params: { credentialIdCan: Ulid.construct(credentialId).toCanonical() },
    to: router.routesById[routes.dashboard.workspace.credential.id].fullPath,
  });

  const content = (
    <>
      {pipe(
        Match.value(kind),
        Match.when(CredentialKind.OPEN_AI, () => <RiOpenaiFill className={tw`size-4 text-on-neutral-low`} />),
        Match.when(CredentialKind.ANTHROPIC, () => <RiAnthropicFill className={tw`size-4 text-on-neutral-low`} />),
        Match.when(CredentialKind.GEMINI, () => <RiGeminiFill className={tw`size-4 text-on-neutral-low`} />),
        Match.orElse(() => <TbGauge className={tw`size-4 text-on-neutral-low`} />),
      )}

      <Text className={twJoin(tw`flex-1 truncate`, isEditing && tw`opacity-0`)} ref={escapeRef}>
        {name}
      </Text>

      {isEditing &&
        escapeRender(
          <TextInputField
            aria-label='Credential name'
            className={tw`w-full`}
            inputClassName={tw`-my-1 py-1`}
            {...textFieldProps}
          />,
        )}

      {showControls && (
        <MenuTrigger {...menuTriggerProps}>
          <Button className={tw`p-0.5`} variant='ghost'>
            <FiMoreHorizontal className={tw`size-4 text-on-neutral-low`} />
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
    </>
  );

  const props = {
    children: content,
    className: toNavigate && matchRoute(route) !== false ? tw`bg-neutral` : '',
    id,
    onContextMenu,
    textValue: name,
  } satisfies TreeItemProps<object>;

  return toNavigate ? <TreeItemRouteLink {...props} {...route} /> : <TreeItem {...props} />;
};
