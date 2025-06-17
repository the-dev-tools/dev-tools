import { ComponentProps } from 'react';
import { twMerge } from 'tailwind-merge';

import { tw } from './tailwind-literal';

// Divider

export const NavigationBarDivider = () => <div className={tw`h-5 w-px bg-white/20`} />;

// Main container

interface NavigationBarProps extends ComponentProps<'div'> {}

export const NavigationBar = ({ className, ...props }: NavigationBarProps) => (
  <div
    className={twMerge(
      tw`
        flex h-12 w-full flex-none items-center gap-4 bg-slate-950 px-4 text-sm font-semibold tracking-tight text-white
      `,
      className,
    )}
    {...props}
  />
);
