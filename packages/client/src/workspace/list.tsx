import { create } from '@bufbuild/protobuf';
import { timestampDate } from '@bufbuild/protobuf/wkt';
import { count, eq, useLiveQuery } from '@tanstack/react-db';
import { DateTime, pipe } from 'effect';
import { Ulid } from 'id128';
import { RefObject, useMemo, useRef } from 'react';
import { Dialog, Heading, ListBox, ListBoxItem, MenuTrigger, useDragAndDrop } from 'react-aria-components';
import { FiMoreHorizontal } from 'react-icons/fi';
import TimeAgo from 'react-timeago';
import { twJoin } from 'tailwind-merge';
import { WorkspaceSchema } from '@the-dev-tools/spec/buf/api/workspace/v1/workspace_pb';
import { FileCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/file_system';
import { WorkspaceCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/workspace';
import { Avatar } from '@the-dev-tools/ui/avatar';
import { Button } from '@the-dev-tools/ui/button';
import { CollectionIcon } from '@the-dev-tools/ui/icons';
import { RouteLink } from '@the-dev-tools/ui/link';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { Modal, useProgrammaticModal } from '@the-dev-tools/ui/modal';
import { DropIndicatorHorizontal } from '@the-dev-tools/ui/reorder';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextInputField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { useEscapePortal } from '@the-dev-tools/ui/utils';
import { useApiCollection } from '~/api';
import { workspaceRouteApi } from '~/routes';
import { getNextOrder, handleCollectionReorder } from '~/utils/order';
import { pick } from '~/utils/tanstack-db';

export const WorkspaceListPage = () => {
  const workspaceCollection = useApiCollection(WorkspaceCollectionSchema);

  const { data: workspaces } = useLiveQuery(
    (_) =>
      _.from({ workspace: workspaceCollection })
        .orderBy((_) => _.workspace.order)
        .select((_) => pick(_.workspace, 'workspaceId', 'name', 'order')),
    [workspaceCollection],
  );

  const containerRef = useRef<HTMLDivElement>(null);

  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) => [...keys].map((key) => ({ key: key.toString() })),
    onReorder: handleCollectionReorder(workspaceCollection),
    renderDropIndicator: () => <DropIndicatorHorizontal />,
  });

  return (
    <div className={tw`container mx-auto my-12 grid min-h-0 gap-x-10 gap-y-6`}>
      <div className={tw`col-span-full`}>
        <span className={tw`mb-1 text-sm leading-5 tracking-tight text-slate-500`}>
          {pipe(DateTime.unsafeNow(), DateTime.formatLocal({ dateStyle: 'full' }))}
        </span>
        <h1 className={tw`text-2xl leading-8 font-medium tracking-tight text-slate-800`}>Welcome to DevTools 👋</h1>
      </div>

      <div className={tw`relative flex min-h-0 flex-col rounded-lg border border-slate-200`} ref={containerRef}>
        <div className={tw`flex items-center gap-2 border-b border-inherit px-5 py-3`}>
          <span className={tw`flex-1 font-semibold tracking-tight text-slate-800`}>Your Workspaces</span>
          <Button
            onPress={async () =>
              void workspaceCollection.utils.insert({
                name: 'New Workspace',
                order: await getNextOrder(workspaceCollection),
                workspaceId: Ulid.generate().bytes,
              })
            }
            variant='primary'
          >
            Add Workspace
          </Button>
        </div>

        <ListBox
          aria-label='Workspaces'
          className={tw`flex-1 divide-y divide-slate-200 overflow-auto`}
          dragAndDropHooks={dragAndDropHooks}
          items={workspaces}
          selectionMode='none'
        >
          {(_) => <Item containerRef={containerRef} id={workspaceCollection.utils.getKey(_)} />}
        </ListBox>
      </div>
    </div>
  );
};

interface ItemProps {
  containerRef: RefObject<HTMLDivElement | null>;
  id: string;
}

const Item = ({ containerRef, id }: ItemProps) => {
  const workspaceCollection = useApiCollection(WorkspaceCollectionSchema);

  const workspaceUlid = useMemo(
    () => pipe(workspaceCollection.utils.parseKeyUnsafe(id), (_) => Ulid.construct(_.workspaceId)),
    [id, workspaceCollection.utils],
  );

  const { name, updated } =
    useLiveQuery(
      (_) =>
        _.from({ workspace: workspaceCollection })
          .where((_) => eq(_.workspace.workspaceId, workspaceUlid.bytes))
          .select((_) => pick(_.workspace, 'name', 'updated'))
          .findOne(),
      [workspaceCollection, workspaceUlid],
    ).data ?? create(WorkspaceSchema);

  const fileCollection = useApiCollection(FileCollectionSchema);

  const { fileCount = 0 } =
    useLiveQuery(
      (_) =>
        _.from({ file: fileCollection })
          .where((_) => eq(_.file.workspaceId, workspaceUlid.bytes))
          .select((_) => ({ fileCount: count(_.file.fileId) }))
          .findOne(),
      [fileCollection, workspaceUlid],
    ).data ?? {};

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const { escapeRef, escapeRender } = useEscapePortal(containerRef);

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => workspaceCollection.utils.update({ name: _, workspaceId: workspaceUlid.bytes }),
    value: name,
  });

  const deleteModal = useProgrammaticModal();

  return (
    <ListBoxItem id={id} textValue={name}>
      {deleteModal.children && <Modal {...deleteModal} className={tw`h-auto`} size='xs' />}

      <div className={tw`flex items-center gap-3 px-5 py-4`} onContextMenu={onContextMenu}>
        <Avatar shape='square' size='md'>
          {name}
        </Avatar>

        <div
          className={tw`
            grid flex-1 grid-flow-col grid-cols-[1fr] grid-rows-2 gap-x-9 text-xs leading-5 tracking-tight
            text-slate-500
          `}
        >
          <div
            className={twJoin(
              tw`text-md leading-5 font-semibold tracking-tight text-slate-800`,
              isEditing && tw`opacity-0`,
            )}
            ref={escapeRef}
          >
            <RouteLink params={{ workspaceIdCan: workspaceUlid.toCanonical() }} to={workspaceRouteApi.id}>
              {name}
            </RouteLink>
          </div>

          {isEditing &&
            escapeRender(
              <TextInputField
                aria-label='Workspace name'
                className={tw`justify-self-start`}
                inputClassName={tw`-mt-1 py-1 text-md leading-none font-semibold tracking-tight text-slate-800`}
                {...textFieldProps}
              />,
            )}

          <div className={tw`flex items-center gap-2`}>
            <span>
              Created <TimeAgo date={workspaceUlid.time} minPeriod={60} />
            </span>
            {updated && (
              <>
                <div className={tw`size-0.5 rounded-full bg-slate-400`} />
                <span>
                  Updated <TimeAgo date={timestampDate(updated)} minPeriod={60} />
                </span>
              </>
            )}
          </div>
          <span>Files</span>
          <div className={tw`flex items-center gap-1`}>
            <CollectionIcon />
            <strong className={tw`font-semibold text-slate-800`}>{fileCount}</strong>
          </div>
        </div>

        <MenuTrigger {...menuTriggerProps}>
          <Button className={tw`ml-6 p-1`} variant='ghost'>
            <FiMoreHorizontal className={tw`size-4 stroke-[1.2px] text-slate-500`} />
          </Button>

          <Menu {...menuProps}>
            <MenuItem onAction={() => void edit()}>Rename</MenuItem>

            <MenuItem
              onAction={() =>
                void deleteModal.onOpenChange(
                  true,
                  <Dialog className={tw`flex flex-col p-5 outline-hidden`}>
                    <Heading
                      className={tw`text-base leading-5 font-semibold tracking-tight text-slate-800`}
                      slot='title'
                    >
                      Delete workspace?
                    </Heading>

                    <div className={tw`mt-1 text-sm leading-5 font-medium tracking-tight text-slate-500`}>
                      Are you sure you want to delete <span className={tw`text-slate-800`}>“{name}”</span>? This action
                      cannot be undone.
                    </div>

                    <div className={tw`mt-5 flex justify-end gap-2`}>
                      <Button slot='close'>Cancel</Button>

                      <Button
                        onPress={() => void workspaceCollection.utils.delete({ workspaceId: workspaceUlid.bytes })}
                        slot='close'
                        variant='danger'
                      >
                        Delete
                      </Button>
                    </div>
                  </Dialog>,
                )
              }
              variant='danger'
            >
              Delete
            </MenuItem>
          </Menu>
        </MenuTrigger>
      </div>
    </ListBoxItem>
  );
};
