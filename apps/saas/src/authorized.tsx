import { createFileRoute, Outlet, redirect } from '@tanstack/react-router';
import { Effect, Option, pipe } from 'effect';
import { MenuTrigger } from 'react-aria-components';

import { getUser } from '@the-dev-tools/api/auth';
import * as Auth from '@the-dev-tools/api/auth';
import { Button, ButtonAsLink } from '@the-dev-tools/ui/button';
import { Menu, MenuItem } from '@the-dev-tools/ui/menu';
import { MixinProps, splitProps } from '@the-dev-tools/utils/mixin-props';

import { LoginSearch } from './login';
import { Runtime } from './runtime';

export const Route = createFileRoute('/_authorized')({
  beforeLoad: ({ location }) =>
    pipe(Effect.option(getUser), Runtime.runPromise, async (_) =>
      Option.getOrThrowWith(await _, () =>
        redirect({
          to: '/login',
          search: new LoginSearch({ redirect: location.href }),
        }),
      ),
    ),
  pendingComponent: () => 'Loading user...',
});

export interface DashboardLayoutProps
  extends MixinProps<'left', React.ComponentProps<'div'>>,
    MixinProps<'right', React.ComponentProps<'div'>> {
  children?: React.ReactNode;
}

export const DashboardLayout = ({ children, ...mixProps }: DashboardLayoutProps) => {
  const props = splitProps(mixProps, 'left', 'right');
  const { email } = Route.useRouteContext();
  return (
    <div className='flex h-full flex-col'>
      <div className='flex items-center gap-2 bg-black p-2 text-white'>
        <ButtonAsLink className='rounded-full' href={{ to: '/' }}>
          DevTools
        </ButtonAsLink>
        {props.left.children && <div {...props.left}>{props.left.children}</div>}
        <div className='flex-1' />
        {props.right.children && <div {...props.right}>{props.right.children}</div>}
        <MenuTrigger>
          <Button className='size-8 rounded-full uppercase'>{email[0]}</Button>
          <Menu>
            <MenuItem isDisabled>User: {email}</MenuItem>
            <MenuItem
              onAction={async () => {
                await Auth.logout.pipe(Effect.ignoreLogged, Runtime.runPromise);
                queueMicrotask(() => void location.reload());
              }}
            >
              Log out
            </MenuItem>
          </Menu>
        </MenuTrigger>
      </div>
      {children ?? <Outlet />}
    </div>
  );
};
