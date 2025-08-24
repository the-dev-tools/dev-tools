import { pipe, Record, Struct } from 'effect';
import { ComponentProps } from 'react';
import { Button as AriaButton, ButtonProps as AriaButtonProps } from 'react-aria-components';
import { tv, VariantProps } from 'tailwind-variants';
import { isFocusVisibleRingStyles } from './focus-ring';
import { tw } from './tailwind-literal';
import { composeRenderPropsTV } from './utils';

interface SharedProps {
  children: string;
  shorten?: boolean;
}

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
  extends Omit<ComponentProps<'div'>, keyof SharedProps>,
    SharedProps,
    VariantProps<typeof avatarStyles> {}

export const Avatar = ({ children, className, shorten = true, ...props }: AvatarProps) => {
  const forwardedProps = Struct.omit(props, ...avatarStyles.variantKeys);
  const variantProps = Struct.pick(props, ...avatarStyles.variantKeys);

  const text = shorten ? children[0]?.toUpperCase() : children;

  return (
    <div {...forwardedProps} className={avatarStyles({ ...variantProps, className })}>
      {text}
    </div>
  );
};

// Button

export const avatarButtonStyles = tv({
  extend: isFocusVisibleRingStyles,
  base: avatarStyles.base,
  variants: {
    ...isFocusVisibleRingStyles.variants,
    ...avatarStyles.variants,
  },
  defaultVariants: avatarStyles.defaultVariants,
});

export const avatarButtonVariantKeys = pipe(
  Struct.omit(avatarButtonStyles.variants, ...isFocusVisibleRingStyles.variantKeys),
  Record.keys,
);

export interface AvatarButtonProps
  extends Omit<AriaButtonProps, keyof SharedProps>,
    SharedProps,
    VariantProps<typeof avatarButtonStyles> {}

export const AvatarButton = ({ children, className, shorten = true, ...props }: AvatarButtonProps) => {
  const forwardedProps = Struct.omit(props, ...avatarButtonVariantKeys);
  const variantProps = Struct.pick(props, ...avatarButtonVariantKeys);

  const text = shorten ? children[0]?.toUpperCase() : children;

  return (
    <AriaButton {...forwardedProps} className={composeRenderPropsTV(className, avatarButtonStyles, variantProps)}>
      {text}
    </AriaButton>
  );
};
