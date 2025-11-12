'use no memo'; // TODO: fix collection tree incorrect first render with compiler

import { useLiveQuery } from '@tanstack/react-db';
import { Outlet, ToOptions } from '@tanstack/react-router';
import { Config, pipe, Runtime } from 'effect';
import { idEqual, Ulid } from 'id128';
import { MenuTrigger, Tooltip, TooltipTrigger } from 'react-aria-components';
import { FiPlus } from 'react-icons/fi';
import { Panel, PanelGroup } from 'react-resizable-panels';
import { WorkspaceCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/workspace';
import { Avatar } from '@the-dev-tools/ui/avatar';
import { Button, ButtonAsLink } from '@the-dev-tools/ui/button';
import { CollectionIcon, OverviewIcon } from '@the-dev-tools/ui/icons';
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { RouteTabList } from '@the-dev-tools/ui/router';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useApiCollection } from '~/api-new';
import { FileCreateMenu, FileTree } from '~/file-system';
import { rootRouteApi, workspaceRouteApi } from '~/routes';
import { pick } from '~/utils/tanstack-db';
import { DashboardLayout } from '../dashboard';
import { EnvironmentsWidget } from '../environment';
import { StatusBar } from '../status-bar';

export const WorkspaceLayout = () => {
  const { runtime } = rootRouteApi.useRouteContext();

  const { workspaceId } = workspaceRouteApi.useLoaderData();
  const { workspaceIdCan } = workspaceRouteApi.useParams();
  const context = workspaceRouteApi.useRouteContext();

  const workspaceCollection = useApiCollection(WorkspaceCollectionSchema);

  const { data: workspace } = useLiveQuery((_) =>
    _.from({ workspace: workspaceCollection })
      .fn.where((_) => idEqual(Ulid.construct(_.workspace.workspaceId), Ulid.construct(workspaceId)))
      .select((_) => pick(_.workspace, 'name'))
      .findOne(),
  );

  if (!workspace) return null;

  const baseRoute: ToOptions = { params: { workspaceIdCan }, to: workspaceRouteApi.id };

  return (
    <DashboardLayout
      navbar={
        <>
          <ButtonAsLink
            className={tw`-ml-3 gap-2 px-2 py-1`}
            params={{ workspaceIdCan }}
            to={workspaceRouteApi.id}
            variant='ghost dark'
          >
            <Avatar shape='square' size='base'>
              {workspace.name}
            </Avatar>
            <span className={tw`text-xs leading-5 font-semibold tracking-tight`}>{workspace.name}</span>
          </ButtonAsLink>

          <div className='flex-1' />
        </>
      }
    >
      <PanelGroup autoSaveId='workspace-sidebar' direction='horizontal'>
        <Panel
          className={tw`flex flex-col bg-slate-50`}
          defaultSize={20}
          maxSize={40}
          minSize={10}
          style={{ overflowY: 'auto' }}
        >
          <EnvironmentsWidget />

          <div className={tw`flex flex-1 flex-col gap-2 overflow-auto p-1.5`}>
            <ButtonAsLink
              className={tw`flex items-center justify-start gap-2 px-2.5 py-1.5`}
              params={{ workspaceIdCan }}
              to={workspaceRouteApi.id}
              variant='ghost'
            >
              <OverviewIcon className={tw`size-5 text-slate-500`} />
              <h2 className={tw`text-md leading-5 font-semibold tracking-tight text-slate-800`}>Overview</h2>
            </ButtonAsLink>

            <div className={tw`flex items-center gap-2 px-2.5 py-1.5`}>
              <CollectionIcon className={tw`size-5 text-slate-500`} />
              <h2 className={tw`flex-1 text-md leading-5 font-semibold tracking-tight text-slate-800`}>Files</h2>

              <MenuTrigger>
                <TooltipTrigger delay={750}>
                  <Button className={tw`bg-slate-200 p-0.5`} variant='ghost'>
                    <FiPlus className={tw`size-4 stroke-[1.2px] text-slate-500`} />
                  </Button>
                  <Tooltip className={tw`rounded-md bg-slate-800 px-2 py-1 text-xs text-white`}>Add New File</Tooltip>
                </TooltipTrigger>

                <FileCreateMenu />
              </MenuTrigger>
            </div>

            <FileTree navigate showControls />
          </div>

          <div className={tw`px-2.5 py-1.5 text-md leading-5 tracking-tight text-slate-800`}>
            DevTools v{pipe(Config.string('VERSION'), Config.withDefault('[DEV]'), Runtime.runSync(runtime))}
          </div>
        </Panel>

        <PanelResizeHandle direction='horizontal' />

        <Panel defaultSize={80}>
          <PanelGroup autoSaveId='workspace-outlet' direction='vertical'>
            <div className={tw`-mt-px pt-2`}>
              <RouteTabList baseRoute={baseRoute} runtime={runtime} tabsAtom={context.tabsAtom} />
            </div>
            <Panel>
              <Outlet />
            </Panel>
            <StatusBar />
          </PanelGroup>
        </Panel>
      </PanelGroup>
    </DashboardLayout>
  );
};

// const FlowList = () => {
//   const { dataClient } = rootRouteApi.useRouteContext();

//   const navigate = useNavigate();

//   const { workspaceId } = workspaceRouteApi.useLoaderData();

//   const { items: flows } = useQuery(FlowListEndpoint, { workspaceId });

//   const listRef = useRef<HTMLDivElement>(null);

//   return (
//     <>
//       <div className={tw`flex items-center gap-2 px-2.5 py-1.5`}>
//         <FlowsIcon className={tw`size-5 text-slate-500`} />
//         <h2 className={tw`flex-1 text-md leading-5 font-semibold tracking-tight text-slate-800`}>Flows</h2>

//         <TooltipTrigger delay={750}>
//           <Button
//             className={tw`bg-slate-200 p-0.5`}
//             onPress={async () => {
//               const { flowId } = await dataClient.fetch(FlowCreateEndpoint, { name: 'New flow', workspaceId });

//               const flowIdCan = Ulid.construct(flowId).toCanonical();

//               await navigate({
//                 from: workspaceRouteApi.id,
//                 to: flowLayoutRouteApi.id,

//                 params: { flowIdCan },
//               });
//             }}
//             variant='ghost'
//           >
//             <FiPlus className={tw`size-4 stroke-[1.2px] text-slate-500`} />
//           </Button>
//           <Tooltip className={tw`rounded-md bg-slate-800 px-2 py-1 text-xs text-white`}>Add New Flow</Tooltip>
//         </TooltipTrigger>
//       </div>

//       <div className={tw`relative`} ref={listRef}>
//         <ListBox aria-label='Flow list' className={tw`w-full`} items={flows} selectionMode='single'>
//           {(_) => {
//             const id = Ulid.construct(_.flowId).toCanonical();
//             return <FlowItem flow={_} id={id} listRef={listRef} />;
//           }}
//         </ListBox>
//       </div>
//     </>
//   );
// };

// interface FlowItemProps {
//   flow: FlowListItem;
//   id: string;
//   listRef: RefObject<HTMLDivElement | null>;
// }

// const FlowItem = ({ flow: { flowId, name }, id: flowIdCan, listRef }: FlowItemProps) => {
//   const { dataClient } = rootRouteApi.useRouteContext();

//   const { workspaceIdCan } = workspaceRouteApi.useParams();
//   const { workspaceId } = workspaceRouteApi.useLoaderData();

//   const onFlowDelete = useOnFlowDelete();

//   const [flowUpdate, flowUpdateLoading] = useMutate(FlowUpdateEndpoint);

//   // TODO: switch to Data Client Endpoint
//   const exportMutation = useConnectMutation(export$);

//   const { menuProps, menuTriggerProps, onContextMenu } = useContextMenuState();

//   const { escapeRef, escapeRender } = useEscapePortal(listRef);

//   const { edit, isEditing, textFieldProps } = useEditableTextState({
//     onSuccess: (_) => flowUpdate({ flowId, name: _ }),
//     value: name,
//   });

//   return (
//     <ListBoxItemLink
//       className={tw`rounded-md pl-9 text-md leading-5 font-medium`}
//       id={flowIdCan}
//       params={{ flowIdCan, workspaceIdCan }}
//       textValue={name}
//       to={flowLayoutRouteApi.id}
//     >
//       <div className={tw`contents`} onContextMenu={onContextMenu}>
//         <Text className={twJoin(tw`flex-1 truncate`, isEditing && tw`opacity-0`)} ref={escapeRef} slot='label'>
//           {name}
//         </Text>

//         {isEditing &&
//           escapeRender(
//             <TextInputField
//               aria-label='Flow name'
//               className={tw`w-full`}
//               inputClassName={tw`-my-1 py-1`}
//               isDisabled={flowUpdateLoading}
//               {...textFieldProps}
//             />,
//           )}

//         <MenuTrigger {...menuTriggerProps}>
//           <Button className={tw`p-0.5`} variant='ghost'>
//             <FiMoreHorizontal className={tw`size-4 text-slate-500`} />
//           </Button>

//           <Menu {...menuProps}>
//             <MenuItem onAction={() => void edit()}>Rename</MenuItem>

//             <MenuItem
//               onAction={async () => {
//                 const { data, name } = await exportMutation.mutateAsync({ flowIds: [flowId], workspaceId });
//                 saveFile({ blobParts: [data], name });
//               }}
//             >
//               Export
//             </MenuItem>

//             <MenuItem
//               onAction={async () => {
//                 await onFlowDelete(flowId);
//                 await dataClient.fetch(FlowDeleteEndpoint, { flowId });
//               }}
//               variant='danger'
//             >
//               Delete
//             </MenuItem>
//           </Menu>
//         </MenuTrigger>
//       </div>
//     </ListBoxItemLink>
//   );
// };
