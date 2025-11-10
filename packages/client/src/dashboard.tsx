import { Outlet } from '@tanstack/react-router';
import { Suspense } from 'react';
import { ButtonAsLink } from '@the-dev-tools/ui/button';
import { Logo } from '@the-dev-tools/ui/illustrations';
import { Spinner } from '@the-dev-tools/ui/spinner';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { welcomeRouteApi } from '~/routes';

export interface DashboardLayoutProps {
  children?: React.ReactNode;
  navbar?: React.ReactNode;
}

export const DashboardLayout = ({ children, navbar }: DashboardLayoutProps) => (
  <div className={tw`flex h-full flex-col`}>
    <div
      className={tw`
        flex h-12 w-full flex-none items-center gap-4 bg-slate-950 px-4 text-sm font-semibold tracking-tight text-white
      `}
    >
      <ButtonAsLink className={tw`p-0`} to={welcomeRouteApi.id} variant='ghost'>
        <Logo className={tw`size-7`} />
      </ButtonAsLink>

      <div className={tw`h-5 w-px bg-white/20`} />

      {navbar}

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
