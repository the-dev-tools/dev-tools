import { ReactNode } from 'react';
import { Button as AriaButton, ButtonProps as AriaButtonProps } from 'react-aria-components';
import { FiPlus } from 'react-icons/fi';
import { twMerge } from 'tailwind-merge';
import { tv } from 'tailwind-variants';

import { avatarStyles } from './avatar';
import { isFocusVisibleRingStyles } from './focus-ring';
import { Logo } from './illustrations';
import { tw } from './tailwind-literal';
import { composeRenderPropsTV } from './utils';

// Divider

export const NavigationBarDivider = () => <div className={tw`h-5 w-px bg-white/20`} />;

// Add member button

const navigationAddMemberButtonStyles = tv({
  extend: isFocusVisibleRingStyles,
  base: twMerge(avatarStyles.base, tw`border-white/20 bg-transparent text-white`),
  variants: {
    isHovered: { true: tw`border-white/40` },
    isPressed: { true: tw`border-white` },
  },
});

interface NavigationAddMemberButtonProps extends Omit<AriaButtonProps, 'children'> {}

export const NavigationAddMemberButton = ({ className, ...props }: NavigationAddMemberButtonProps) => (
  <AriaButton {...props} className={composeRenderPropsTV(className, navigationAddMemberButtonStyles)}>
    <FiPlus className='size-4 stroke-[1.2px]' />
  </AriaButton>
);

// Main container

interface NavigationBarProps {
  children?: ReactNode;
}

export const NavigationBar = ({ children }: NavigationBarProps) => (
  <div
    className={tw`flex h-12 w-full items-center gap-4 bg-slate-950 px-4 text-sm font-semibold tracking-tight text-white`}
  >
    <Logo className='size-7' />
    <NavigationBarDivider />
    {children}
  </div>
);
