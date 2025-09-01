import { timestampDate } from '@bufbuild/protobuf/wkt';
import { DateTime, pipe } from 'effect';
import { Ulid } from 'id128';
import { RefObject, useRef } from 'react';
import { ListBox, ListBoxItem, MenuTrigger, useDragAndDrop } from 'react-aria-components';
import { FiMoreHorizontal } from 'react-icons/fi';
import TimeAgo from 'react-timeago';
import { twJoin } from 'tailwind-merge';
import {
  WorkspaceCreateEndpoint,
  WorkspaceDeleteEndpoint,
  WorkspaceListEndpoint,
  WorkspaceMoveEndpoint,
  WorkspaceUpdateEndpoint,
} from '@the-dev-tools/spec/meta/workspace/v1/workspace.endpoints.ts';
import { WorkspaceListItemEntity } from '@the-dev-tools/spec/meta/workspace/v1/workspace.entities.js';
import { Avatar } from '@the-dev-tools/ui/avatar';
import { Button } from '@the-dev-tools/ui/button';
import { CollectionIcon, FlowsIcon } from '@the-dev-tools/ui/icons';
import { Link } from '@the-dev-tools/ui/link';
import { Menu, MenuItem, useContextMenuState } from '@the-dev-tools/ui/menu';
import { basicReorder, DropIndicatorHorizontal } from '@the-dev-tools/ui/reorder';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { TextField, useEditableTextState } from '@the-dev-tools/ui/text-field';
import { useEscapePortal } from '@the-dev-tools/ui/utils';
import { useMutate, useQuery } from '~data-client';
import { rootRouteApi, workspaceRouteApi } from '~routes';

export const WorkspaceListPage = () => {
  const { dataClient } = rootRouteApi.useRouteContext();

  const { items: workspaces } = useQuery(WorkspaceListEndpoint, {});

  const containerRef = useRef<HTMLDivElement>(null);

  const { dragAndDropHooks } = useDragAndDrop({
    getItems: (keys) => [...keys].map((key) => ({ key: key.toString() })),
    onReorder: basicReorder(({ position, source, target }) =>
      dataClient.fetch(WorkspaceMoveEndpoint, {
        position,
        targetWorkspaceId: Ulid.fromCanonical(target).bytes,
        workspaceId: Ulid.fromCanonical(source).bytes,
      }),
    ),
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
          {/* <Button>View All Workspaces</Button> */}
          <Button
            onPress={() => void dataClient.fetch(WorkspaceCreateEndpoint, { name: 'New Workspace' })}
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
          {(_) => <Item containerRef={containerRef} data={_} id={Ulid.construct(_.workspaceId).toCanonical()} />}
        </ListBox>
      </div>
    </div>
  );
};

interface ItemProps {
  containerRef: RefObject<HTMLDivElement | null>;
  data: WorkspaceListItemEntity;
  id: string;
}

const Item = ({
  containerRef,
  data: { collectionCount, flowCount, name, updated, workspaceId },
  id: workspaceIdCan,
}: ItemProps) => {
  const { dataClient } = rootRouteApi.useRouteContext();

  const [workspaceUpdate, workspaceUpdateLoading] = useMutate(WorkspaceUpdateEndpoint);

  const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

  const escape = useEscapePortal(containerRef);

  const { edit, isEditing, textFieldProps } = useEditableTextState({
    onSuccess: (_) => workspaceUpdate({ name: _, workspaceId }),
    value: name,
  });

  return (
    <ListBoxItem id={workspaceIdCan} textValue={name}>
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
            ref={escape.ref}
          >
            <Link params={{ workspaceIdCan }} to={workspaceRouteApi.id}>
              {name}
            </Link>
          </div>

          {isEditing &&
            escape.render(
              <TextField
                aria-label='Workspace name'
                className={tw`justify-self-start`}
                inputClassName={tw`-mt-1 py-1 text-md leading-none font-semibold tracking-tight text-slate-800`}
                isDisabled={workspaceUpdateLoading}
                {...textFieldProps}
              />,
            )}

          <div className={tw`flex items-center gap-2`}>
            {/* <span>
            by <strong className={tw`font-medium`}>N/A</strong>
          </span> */}
            {/* <div className={tw`size-0.5 rounded-full bg-slate-400`} /> */}
            <span>
              Created <TimeAgo date={Ulid.construct(workspaceId).time} minPeriod={60} />
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
          <span>Collection</span>
          <div className={tw`flex items-center gap-1`}>
            <CollectionIcon />
            <strong className={tw`font-semibold text-slate-800`}>{collectionCount}</strong>
          </div>
          <span>Flows</span>
          <div className={tw`flex items-center gap-1`}>
            <FlowsIcon />
            <strong className={tw`font-semibold text-slate-800`}>{flowCount}</strong>
          </div>
          {/* <span>N/A Members</span> */}
          {/* <div className={tw`flex gap-2`}>
          <div className={tw`flex`}>
            {['A', 'B', 'C', 'D'].map((_) => (
              <Avatar key={_} className={tw`-ml-1.5 first:ml-0`}>
                {_}
              </Avatar>
            ))}
          </div>
          <AddButton />
        </div> */}
        </div>

        <MenuTrigger {...menuTriggerProps}>
          <Button className={tw`ml-6 p-1`} variant='ghost'>
            <FiMoreHorizontal className={tw`size-4 stroke-[1.2px] text-slate-500`} />
          </Button>

          <Menu {...menuProps}>
            <MenuItem onAction={() => void edit()}>Rename</MenuItem>
            <MenuItem onAction={() => void dataClient.fetch(WorkspaceDeleteEndpoint, { workspaceId })} variant='danger'>
              Delete
            </MenuItem>
          </Menu>
        </MenuTrigger>
      </div>
    </ListBoxItem>
  );
};
