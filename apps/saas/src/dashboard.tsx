import { useQuery as useConnectQuery } from '@connectrpc/connect-query';
import { getRouteApi, Link, Outlet, useRouter } from '@tanstack/react-router';
import { Effect } from 'effect';

import * as Auth from '@the-dev-tools/api/auth';
import { getOrganization } from '@the-dev-tools/protobuf/organization/v1/organization-OrganizationService_connectquery';
import { Button } from '@the-dev-tools/ui/button';

import { Runtime } from './runtime';

const route = getRouteApi('/user/org/dashboard');

export const DashboardLayout = () => {
  const router = useRouter();
  const { email, organizationId } = route.useRouteContext();
  const organizationQuery = useConnectQuery(getOrganization, { organizationId });

  return (
    <div className='flex h-full'>
      <div className='flex h-full w-80 flex-col gap-2 overflow-auto border-r-4 border-black p-2'>
        <h1 className='text-center text-2xl font-extrabold'>The Dev Tools</h1>
        <Link to='/'>Dashboard</Link>
        <Link to='/collections'>Collections</Link>
        <div className='flex-1' />
        <div>User: {email}</div>
        {organizationQuery.isSuccess && (
          <Link to='/organizations' search={{ redirect: router.history.location.href }}>
            Organization: {organizationQuery.data.organization!.name}
          </Link>
        )}
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

export const DashboardIndexPage = () => <h2 className='text-center text-2xl font-extrabold'>Dashboard</h2>;
