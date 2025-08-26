import { Outlet } from '@tanstack/react-router';
import { Suspense } from 'react';
import { ButtonAsLink } from '@the-dev-tools/ui/button';
import { Logo } from '@the-dev-tools/ui/illustrations';
import { NavigationBar, NavigationBarDivider } from '@the-dev-tools/ui/navigation-bar';
import { Spinner } from '@the-dev-tools/ui/spinner';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { welcomeRouteApi } from '~routes';

export interface DashboardLayoutProps {
  children?: React.ReactNode;
  navbar?: React.ReactNode;
}

export const DashboardLayout = ({ children, navbar }: DashboardLayoutProps) => (
  <div className='flex h-full flex-col'>
    <NavigationBar>
      <ButtonAsLink className={tw`p-0`} to={welcomeRouteApi.id} variant='ghost'>
        <Logo className={tw`size-7`} />
      </ButtonAsLink>

      <NavigationBarDivider />

      {navbar}

      <a href='https://github.com/the-dev-tools/dev-tools' rel='noreferrer' target='_blank'>
        <img alt='GitHub Repo stars' src='https://img.shields.io/github/stars/the-dev-tools/dev-tools' />
      </a>
    </NavigationBar>

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
