import { pipe, Record, Struct } from 'effect';
import {
  Button as AriaButton,
  Link as AriaLink,
  type ButtonProps as AriaButtonProps,
  type LinkProps as AriaLinkProps,
} from 'react-aria-components';
import { tv, type VariantProps } from 'tailwind-variants';

import { isFocusVisibleRingRenderPropKeys, isFocusVisibleRingStyles } from './focus-ring';
import { tw } from './tailwind-literal';
import { composeRenderPropsTV } from './utils';

export const buttonStyles = tv({
  extend: isFocusVisibleRingStyles,
  base: tw`flex cursor-pointer select-none items-center justify-center gap-1 rounded-md border border-transparent bg-transparent px-4 py-1.5 text-sm font-medium leading-5 tracking-tight`,
  variants: {
    ...isFocusVisibleRingStyles.variants,
    variant: {
      primary: tw`border-violet-700 bg-violet-600 text-white`,
      secondary: tw`border-slate-200 bg-white text-slate-800`,
      ghost: tw`text-slate-800`,
      'ghost dark': tw`text-white`,
    },
    isHovered: { true: null },
    isPressed: { true: null },
    isDisabled: { true: tw`cursor-not-allowed` },
  },
  compoundVariants: [
    { variant: 'primary', isHovered: true, className: tw`border-violet-800 bg-violet-700` },
    { variant: 'primary', isPressed: true, className: tw`border-violet-900 bg-violet-800` },
    { variant: 'primary', isDisabled: true, className: tw`border-violet-400 bg-violet-400` },

    { variant: 'secondary', isHovered: true, className: tw`border-slate-200 bg-slate-100` },
    { variant: 'secondary', isPressed: true, className: tw`border-slate-300 bg-white` },

    { variant: 'ghost', isHovered: true, className: tw`bg-slate-100` },
    { variant: 'ghost', isPressed: true, className: tw`bg-slate-200` },

    { variant: 'ghost dark', isHovered: true, className: tw`bg-slate-600` },
    { variant: 'ghost dark', isPressed: true, className: tw`bg-slate-700` },
  ],
  defaultVariants: {
    variant: 'secondary',
  },
});

const renderPropKeys = [...isFocusVisibleRingRenderPropKeys, 'isHovered', 'isPressed', 'isDisabled'] as const;
export const buttonVariantKeys = pipe(Struct.omit(buttonStyles.variants, ...renderPropKeys), Record.keys);

// Button

export interface ButtonProps extends AriaButtonProps, VariantProps<typeof buttonStyles> {}

export const Button = ({ className, ...props }: ButtonProps) => {
  const forwardedProps = Struct.omit(props, ...buttonVariantKeys);
  const variantProps = Struct.pick(props, ...buttonVariantKeys);
  return <AriaButton {...forwardedProps} className={composeRenderPropsTV(className, buttonStyles, variantProps)} />;
};

// Button as link

export interface ButtonAsLinkProps extends AriaLinkProps, VariantProps<typeof buttonStyles> {}

export const ButtonAsLink = ({ className, ...props }: ButtonAsLinkProps) => {
  const forwardedProps = Struct.omit(props, ...buttonVariantKeys);
  const variantProps = Struct.pick(props, ...buttonVariantKeys);
  return <AriaLink {...forwardedProps} className={composeRenderPropsTV(className, buttonStyles, variantProps)} />;
};
