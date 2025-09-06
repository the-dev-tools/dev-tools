import { ComponentProps } from 'react';
import * as RAC from 'react-aria-components';
import { tv, VariantProps } from 'tailwind-variants';
import { focusVisibleRingStyles } from './focus-ring';
import { tw } from './tailwind-literal';
import { composeStyleProps } from './utils';

// Text

interface AvatarTextProps {
  children: string;
  shorten?: boolean;
}

const AvatarText = ({ children, shorten = true }: AvatarTextProps) => (shorten ? children[0]?.toUpperCase() : children);

// Main

export const avatarStyles = tv({
  base: tw`flex items-center justify-center border font-semibold select-none`,
  variants: {
    shape: {
      circle: tw`rounded-full`,
      square: tw`rounded-md`,
    },
    size: {
      base: tw`size-7`,
      md: tw`size-9`,
      sm: tw`size-5 text-[0.625rem]`,
    },
    variant: {
      amber: tw`border-amber-500 bg-amber-100 text-amber-600`,
      blue: tw`border-blue-400 bg-blue-100 text-blue-600`,
      lime: tw`border-lime-500 bg-lime-200 text-lime-600`,
      neutral: tw`border-slate-200 bg-white text-slate-800`,
      pink: tw`border-pink-400 bg-pink-100 text-pink-600`,
      teal: tw`border-teal-400 bg-teal-100 text-teal-600`,
      violet: tw`border-violet-400 bg-violet-200 text-violet-600`,
    },
  },
  defaultVariants: {
    shape: 'circle',
    size: 'sm',
    variant: 'neutral',
  },
});

export interface AvatarProps
  extends AvatarTextProps,
    Omit<ComponentProps<'div'>, keyof AvatarTextProps>,
    VariantProps<typeof avatarStyles> {}

export const Avatar = ({ children, ...props }: AvatarProps) => (
  <div {...props} className={avatarStyles(props)}>
    <AvatarText {...props}>{children}</AvatarText>
  </div>
);

// Button

export const avatarButtonStyles = tv({
  extend: avatarStyles,
  base: focusVisibleRingStyles(),
});

export interface AvatarButtonProps
  extends AvatarTextProps,
    Omit<RAC.ButtonProps, keyof AvatarTextProps>,
    VariantProps<typeof avatarButtonStyles> {}

export const AvatarButton = ({ children, ...props }: AvatarButtonProps) => (
  <RAC.Button {...props} className={composeStyleProps(props, avatarButtonStyles)}>
    <AvatarText {...props}>{children}</AvatarText>
  </RAC.Button>
);
