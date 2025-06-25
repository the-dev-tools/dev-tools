import { createFileRoute, Outlet, redirect } from '@tanstack/react-router';
import { Effect, Option, pipe, Runtime } from 'effect';
import { Suspense } from 'react';

import { ButtonAsLink } from '@the-dev-tools/ui/button';
import { Spinner } from '@the-dev-tools/ui/icons';
import { Logo } from '@the-dev-tools/ui/illustrations';
import { NavigationBar, NavigationBarDivider } from '@the-dev-tools/ui/navigation-bar';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { getUser } from '~/api/auth';

import { LoginSearch } from './login';

const makeRoute = createFileRoute('/_authorized');

export const Route = makeRoute({
  beforeLoad: ({ context: { runtime }, location }) =>
    pipe(Effect.option(getUser), Runtime.runPromise(runtime), async (_) =>
      Option.getOrThrowWith(await _, () =>
        redirect({
          search: LoginSearch.make({ redirect: location.href }),
          to: '/login',
        }),
      ),
    ),
});

export interface DashboardLayoutProps {
  children?: React.ReactNode;
  navbar?: React.ReactNode;
}

export const DashboardLayout = ({ children, navbar }: DashboardLayoutProps) => (
  <div className='flex h-full flex-col'>
    <NavigationBar>
      <ButtonAsLink className={tw`p-0`} from='/' to='/' variant='ghost'>
        <Logo className={tw`size-7`} />
      </ButtonAsLink>

      <NavigationBarDivider />

      {navbar}

      {/* <NavigationBarDivider />

        <MenuTrigger>
          <AvatarButton size='base'>{email}</AvatarButton>

          <Menu>
            <MenuItem isDisabled>User: {email}</MenuItem>
            <MenuItem
              onAction={async () => {
                await Auth.logout.pipe(Effect.ignoreLogged, Runtime.runPromise(runtime));
                queueMicrotask(() => void location.reload());
              }}
            >
              Log out
            </MenuItem>
          </Menu>
        </MenuTrigger> */}
    </NavigationBar>

    <Suspense
      fallback={
        <div className={tw`flex h-full items-center justify-center`}>
          <Spinner className={tw`size-16`} />
        </div>
      }
    >
      {children ?? <Outlet />}
    </Suspense>
  </div>
);
