import { getRouteApi, Outlet } from '@tanstack/react-router';
import { Effect } from 'effect';

import * as Auth from '@the-dev-tools/api/auth';
import { Button } from '@the-dev-tools/ui/button';

import { Runtime } from './runtime';

const authenticatedRoute = getRouteApi('/authenticated');

export const DashboardLayout = () => {
  const { email } = authenticatedRoute.useLoaderData();
  return (
    <div className='flex h-full'>
      <div className='flex h-full w-80 flex-col overflow-auto border-r-4 border-black p-2'>
        <h1 className='text-center text-2xl font-extrabold'>The Dev Tools</h1>
        <div className='flex-1' />
        <div className='mb-1'>User: {email}</div>
        <Button
          onPress={async () => {
            await Auth.logout.pipe(Effect.ignoreLogged, Runtime.runPromise);
            queueMicrotask(() => void location.reload());
          }}
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
