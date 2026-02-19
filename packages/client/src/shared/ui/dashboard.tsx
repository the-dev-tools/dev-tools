import { Result, useAtomValue } from '@effect-atom/atom-react';
import { Outlet, useRouter } from '@tanstack/react-router';
import { Option, pipe, Runtime } from 'effect';
import { Suspense } from 'react';
import { MenuTrigger } from 'react-aria-components';
import { FiMoon, FiSun } from 'react-icons/fi';
import { AvatarButton } from '@the-dev-tools/ui/avatar';
import { Button, ButtonAsRouteLink } from '@the-dev-tools/ui/button';
import { Logo } from '@the-dev-tools/ui/illustrations';
import { Menu, MenuItem } from '@the-dev-tools/ui/menu';
import { Spinner } from '@the-dev-tools/ui/spinner';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { useTheme } from '@the-dev-tools/ui/theme';
import { runtimeAtom } from '~/shared/lib';
import { AuthData, AuthService } from '../api';
import { routes } from '../routes';

interface DashboardLayoutProps {
  children?: React.ReactNode;
  navbar?: React.ReactNode;
}

const authAtom = runtimeAtom.atom(AuthService.getAuthData);

export const DashboardLayout = ({ children, navbar }: DashboardLayoutProps) => {
  const router = useRouter();

  const { runtime } = routes.root.useRouteContext();

  const { theme, toggleTheme } = useTheme();

  const user = pipe(
    useAtomValue(authAtom),
    Result.getOrElse(() => Option.none<AuthData>()),
    Option.match({
      onNone: () => (
        <ButtonAsRouteLink
          className={tw`px-2.5 py-1`}
          to={router.routesById[routes.dashboard.workspace.user.signIn.id].fullPath}
          variant='secondary'
        >
          Sign In
        </ButtonAsRouteLink>
      ),
      onSome: (_) => (
        <MenuTrigger>
          <AvatarButton>{_.name}</AvatarButton>

          <Menu>
            <MenuItem onAction={() => void Runtime.runPromise(runtime, AuthService.signOut)}>Sign Out</MenuItem>
          </Menu>
        </MenuTrigger>
      ),
    }),
  );

  return (
    <div className={tw`flex h-full flex-col`}>
      <div
        className={tw`
          flex h-12 w-full flex-none items-center gap-4 bg-inverse px-4 text-sm font-semibold tracking-tight
          text-on-inverse
        `}
      >
        <ButtonAsRouteLink
          className={tw`p-0`}
          to={router.routesById[routes.dashboard.index.id].fullPath}
          variant='ghost'
        >
          <Logo className={tw`size-7`} />
        </ButtonAsRouteLink>

        <div className={tw`h-5 w-px bg-on-inverse-lower`} />

        {navbar}

        <div className='flex-1' />

        <Button className={tw`-mr-2 p-1 text-xl`} onPress={() => void toggleTheme()} variant='ghost dark'>
          {theme === 'light' && <FiSun />}
          {theme === 'dark' && <FiMoon />}
        </Button>

        <div className={tw`h-5 w-px bg-on-inverse-lower`} />

        {user}

        <div className={tw`h-5 w-px bg-on-inverse-lower`} />

        <a href='https://github.com/the-dev-tools/dev-tools' rel='noreferrer' target='_blank'>
          <img alt='GitHub Repo stars' src='https://img.shields.io/github/stars/the-dev-tools/dev-tools' />
        </a>
      </div>

      <Suspense
        fallback={
          <div className={tw`flex h-full items-center justify-center`}>
            <Spinner size='xl' />
          </div>
        }
      >
        {children ?? <Outlet />}
      </Suspense>
    </div>
  );
};
