import { createFileRoute, Outlet } from '@tanstack/react-router';

import { DashboardLayout } from './authorized';

export const Route = createFileRoute('/_authorized/_dashboard')({
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
