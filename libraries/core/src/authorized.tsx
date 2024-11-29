import { createFileRoute, Outlet, redirect } from '@tanstack/react-router';
import { Effect, Option, pipe, Runtime } from 'effect';
import { MenuTrigger } from 'react-aria-components';

import * as Auth from '@the-dev-tools/api/auth';
import { getUser } from '@the-dev-tools/api/auth';
import { AvatarButton } from '@the-dev-tools/ui/avatar';
import { ButtonAsLink } from '@the-dev-tools/ui/button';
import { Logo } from '@the-dev-tools/ui/illustrations';
import { Menu, MenuItem } from '@the-dev-tools/ui/menu';
import { NavigationBar, NavigationBarDivider } from '@the-dev-tools/ui/navigation-bar';
import { tw } from '@the-dev-tools/ui/tailwind-literal';

import { LoginSearch } from './login';

export const Route = createFileRoute('/_authorized')({
  beforeLoad: ({ location, context: { runtime } }) =>
    pipe(Effect.option(getUser), Runtime.runPromise(runtime), async (_) =>
      Option.getOrThrowWith(await _, () =>
        redirect({
          to: '/login',
          search: new LoginSearch({ redirect: location.href }),
        }),
      ),
    ),
  pendingComponent: () => 'Loading user...',
});

export interface DashboardLayoutProps {
  navbar?: React.ReactNode;
  children?: React.ReactNode;
}

export const DashboardLayout = ({ navbar, children }: DashboardLayoutProps) => {
  const { email, runtime } = Route.useRouteContext();
  return (
    <div className='flex h-full flex-col'>
      <NavigationBar>
        <ButtonAsLink href={{ to: '/' }} variant='ghost' className={tw`p-0`}>
          <Logo className={tw`size-7`} />
        </ButtonAsLink>

        <NavigationBarDivider />

        {navbar}

        <NavigationBarDivider />

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
        </MenuTrigger>
      </NavigationBar>

      {children ?? <Outlet />}
    </div>
  );
};
