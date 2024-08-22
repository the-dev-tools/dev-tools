import { getRouteApi, Link, Outlet } from '@tanstack/react-router';
import { Effect } from 'effect';

import * as Auth from '@the-dev-tools/api/auth';
import { Button } from '@the-dev-tools/ui/button';

import { Runtime } from './runtime';

const route = getRouteApi('/_authorized');

export interface DashboardLayoutProps {
  children?: React.ReactNode;
}

export const DashboardLayout = ({ children }: DashboardLayoutProps) => {
  const { email } = route.useRouteContext();

  return (
    <div className='flex h-full'>
      <div className='flex h-full w-80 flex-col gap-2 overflow-auto border-r-4 border-black p-2'>
        <h1 className='text-center text-2xl font-extrabold'>The Dev Tools</h1>
        <Link to='/'>Home</Link>
        <div className='flex-1' />
        {children}
        <div>User: {email}</div>
        <Button
          onPress={async () => {
            await Auth.logout.pipe(Effect.ignoreLogged, Runtime.runPromise);
            queueMicrotask(() => void location.reload());
          }}
          variant='secondary gray'
        >
          Log out
        </Button>
      </div>
      <div className='h-full flex-1 overflow-auto p-2'>
        <Outlet />
      </div>
    </div>
  );
};
