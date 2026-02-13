'use no memo'; // TODO: fix collection tree incorrect first render with compiler

import { useLiveQuery } from '@tanstack/react-db';
import { createFileRoute, Outlet, useRouter } from '@tanstack/react-router';
import { Config, pipe, Runtime, Schema } from 'effect';
import { idEqual, Ulid } from 'id128';
import { MenuTrigger, Tooltip, TooltipTrigger } from 'react-aria-components';
import { FiPlus } from 'react-icons/fi';
import { Panel, Group as PanelGroup, useDefaultLayout } from 'react-resizable-panels';
import { WorkspaceCollectionSchema } from '@the-dev-tools/spec/tanstack-db/v1/api/workspace';
import { Avatar } from '@the-dev-tools/ui/avatar';
import { Button, ButtonAsRouteLink } from '@the-dev-tools/ui/button';
import { CollectionIcon, OverviewIcon } from '@the-dev-tools/ui/icons';
import { PanelResizeHandle } from '@the-dev-tools/ui/resizable-panel';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { FileCreateMenu, FileTree } from '~/features/file-system';
import { useApiCollection } from '~/shared/api';
import { pick } from '~/shared/lib';
import { routes } from '~/shared/routes';
import { DashboardLayout } from '~/shared/ui';
import { EnvironmentsWidget } from '~/widgets/environment';
import { RouteTabList } from '~/widgets/tabs';
import { StatusBar } from '../../../ui/status-bar';

export class WorkspaceRouteSearch extends Schema.Class<WorkspaceRouteSearch>('WorkspaceRouteSearch')({
  showLogs: pipe(Schema.Boolean, Schema.optional),
}) {}

/* eslint-disable perfectionist/sort-objects */
export const Route = createFileRoute('/(dashboard)/(workspace)/workspace/$workspaceIdCan')({
  validateSearch: (_) => Schema.decodeSync(WorkspaceRouteSearch)(_),
  loader: ({ params: { workspaceIdCan } }) => {
    const workspaceId = Ulid.fromCanonical(workspaceIdCan).bytes;
    return { workspaceId };
  },
  component: RouteComponent,
});
/* eslint-enable perfectionist/sort-objects */

function RouteComponent() {
  const router = useRouter();
  const { runtime } = routes.root.useRouteContext();
  const { workspaceId } = routes.dashboard.workspace.route.useLoaderData();
  const { workspaceIdCan } = routes.dashboard.workspace.route.useParams();

  const workspaceCollection = useApiCollection(WorkspaceCollectionSchema);

  const workspace = useLiveQuery(
    (_) =>
      _.from({ workspace: workspaceCollection })
        .fn.where((_) => idEqual(Ulid.construct(_.workspace.workspaceId), Ulid.construct(workspaceId)))
        .select((_) => pick(_.workspace, 'name'))
        .findOne(),
    [workspaceCollection, workspaceId],
  ).data;

  const workspaceSidebarLayout = useDefaultLayout({ id: 'workspace-sidebar' });

  const workspaceOutletLayout = useDefaultLayout({ id: 'workspace-outlet' });

  if (!workspace) return null;

  // const baseRoute: ToOptions = { params: { workspaceIdCan }, to: workspaceRouteApi.id };

  return (
    <DashboardLayout
      navbar={
        <>
          <ButtonAsRouteLink
            className={tw`-ml-3 gap-2 px-2 py-1`}
            params={{ workspaceIdCan }}
            to={router.routesById[routes.dashboard.workspace.route.id].fullPath}
            variant='ghost dark'
          >
            <Avatar shape='square' size='base'>
              {workspace.name}
            </Avatar>
            <span className={tw`text-xs leading-5 font-semibold tracking-tight`}>{workspace.name}</span>
          </ButtonAsRouteLink>

          <div className='flex-1' />
        </>
      }
    >
      <PanelGroup {...workspaceSidebarLayout} orientation='horizontal'>
        <Panel
          className={tw`flex flex-col bg-neutral-lower`}
          defaultSize='20%'
          maxSize='40%'
          minSize='10%'
          style={{ overflowY: 'auto' }}
        >
          <EnvironmentsWidget />

          <div className={tw`flex flex-1 flex-col gap-2 overflow-auto p-1.5`}>
            <ButtonAsRouteLink
              className={tw`flex items-center justify-start gap-2 px-2.5 py-1.5`}
              params={{ workspaceIdCan }}
              to={router.routesById[routes.dashboard.workspace.route.id].fullPath}
              variant='ghost'
            >
              <OverviewIcon className={tw`size-5 text-on-neutral-low`} />
              <h2 className={tw`text-md leading-5 font-semibold tracking-tight text-on-neutral`}>Overview</h2>
            </ButtonAsRouteLink>

            <div className={tw`flex items-center gap-2 px-2.5 py-1.5`}>
              <CollectionIcon className={tw`size-5 text-on-neutral-low`} />
              <h2 className={tw`flex-1 text-md leading-5 font-semibold tracking-tight text-on-neutral`}>Files</h2>

              <MenuTrigger>
                <TooltipTrigger delay={750}>
                  <Button className={tw`bg-neutral p-0.5`} variant='ghost'>
                    <FiPlus className={tw`size-4 stroke-[1.2px] text-on-neutral-low`} />
                  </Button>
                  <Tooltip className={tw`rounded-md bg-inverse px-2 py-1 text-xs text-on-inverse`}>
                    Add New File
                  </Tooltip>
                </TooltipTrigger>

                <FileCreateMenu navigate />
              </MenuTrigger>
            </div>

            <FileTree navigate showControls />
          </div>

          <div className={tw`px-2.5 py-1.5 text-md leading-5 tracking-tight text-on-neutral`}>
            DevTools v{pipe(Config.string('VERSION'), Config.withDefault('[DEV]'), Runtime.runSync(runtime))}
          </div>
        </Panel>

        <PanelResizeHandle direction='horizontal' />

        <Panel className={tw`bg-neutral-lowest`} defaultSize='80%'>
          <PanelGroup {...workspaceOutletLayout} orientation='vertical'>
            <div className={tw`-mt-px pt-2`}>
              <RouteTabList />
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
}
