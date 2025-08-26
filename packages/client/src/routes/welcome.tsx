import { createFileRoute } from '@tanstack/react-router';
import { DashboardLayout } from '~dashboard';
import { WorkspaceListPage } from '~workspace/list';

export const Route = createFileRoute('/')({
  component: RouteComponent,
});

function RouteComponent() {
  return (
    <DashboardLayout
      navbar={
        <>
          <span>Home</span>
          <div className='flex-1' />
        </>
      }
    >
      <WorkspaceListPage />
    </DashboardLayout>
  );
}
