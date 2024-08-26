import { getRouteApi, Outlet } from '@tanstack/react-router';
import { Effect } from 'effect';
import { Button, Link, Menu, MenuItem, MenuTrigger, Popover } from 'react-aria-components';

import * as Auth from '@the-dev-tools/api/auth';
import { MixinProps, splitProps } from '@the-dev-tools/utils/mixin-props';

import { Runtime } from './runtime';

const route = getRouteApi('/_authorized');

export interface DashboardLayoutProps
  extends MixinProps<'left', React.ComponentProps<'div'>>,
    MixinProps<'right', React.ComponentProps<'div'>> {
  children?: React.ReactNode;
}

export const DashboardLayout = ({ children, ...mixProps }: DashboardLayoutProps) => {
  const props = splitProps(mixProps, 'left', 'right');
  const { email } = route.useRouteContext();
  return (
    <div className='flex h-full flex-col'>
      <div className='flex items-center gap-2 bg-black p-2 text-white'>
        <Link
          className='flex h-8 select-none items-center justify-center rounded-full bg-white px-4 text-black'
          href={{ to: '/' }}
        >
          DevTools
        </Link>
        {props.left.children && <div {...props.left}>{props.left.children}</div>}
        <div className='flex-1' />
        {props.right.children && <div {...props.right}>{props.right.children}</div>}
        <MenuTrigger>
          <Button className='flex size-8 items-center justify-center rounded-full bg-white uppercase text-black'>
            {email[0]}
          </Button>
          <Popover>
            <Menu className='flex flex-col gap-2 rounded border-2 border-black bg-white p-2'>
              <MenuItem isDisabled>User: {email}</MenuItem>
              <MenuItem
                onAction={async () => {
                  await Auth.logout.pipe(Effect.ignoreLogged, Runtime.runPromise);
                  queueMicrotask(() => void location.reload());
                }}
                className='cursor-pointer'
              >
                Log out
              </MenuItem>
            </Menu>
          </Popover>
        </MenuTrigger>
      </div>
      {children ? (
        children
      ) : (
        <div className='h-full flex-1 overflow-auto p-2'>
          <Outlet />
        </div>
      )}
    </div>
  );
};

export const DashboardRootLayout = () => <DashboardLayout leftChildren='Home' />;
