import { createFileRoute, Outlet } from '@tanstack/react-router';

import { DashboardLayout } from './authorized';

const makeRoute = createFileRoute('/_authorized/_dashboard');

export const Route = makeRoute({
  component: () => (
    <DashboardLayout
      navbar={
        <>
          <span>Home</span>
          <div className='flex-1' />
        </>
      }
    >
      <Outlet />
    </DashboardLayout>
  ),
});
