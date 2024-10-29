import { Struct } from 'effect';
import {
  Button as AriaButton,
  Link as AriaLink,
  type ButtonProps as AriaButtonProps,
  type LinkProps as AriaLinkProps,
} from 'react-aria-components';
import { tv, type VariantProps } from 'tailwind-variants';

import { focusRingStyles } from './focus-ring';
import { tw } from './tailwind-literal';
import { composeRenderPropsTV } from './utils';

// Button

export const buttonStyles = tv({
  extend: focusRingStyles,
  base: tw`flex cursor-pointer select-none items-center justify-center gap-0.5 rounded border px-3 py-2 text-sm leading-none text-black`,
  variants: {
    variant: {
      default: tw`border-black bg-neutral-200`,
      ghost: tw`border-transparent bg-transparent p-1`,
    },
    isHovered: { true: null },
  },
  compoundVariants: [
    {
      variant: 'default',
      isHovered: true,
      className: tw`bg-neutral-400`,
    },
  ],
  defaultVariants: {
    variant: 'default',
  },
});

export interface ButtonProps extends AriaButtonProps, VariantProps<typeof buttonStyles> {}

export const Button = ({ className, ...props }: ButtonProps) => {
  const forwardedProps = Struct.omit(props, ...buttonStyles.variantKeys);
  const variantProps = Struct.pick(props, ...buttonStyles.variantKeys);
  return <AriaButton {...forwardedProps} className={composeRenderPropsTV(className, buttonStyles, variantProps)} />;
};

// Button as link

export interface ButtonAsLinkProps extends AriaLinkProps, VariantProps<typeof buttonStyles> {}

export const ButtonAsLink = ({ className, ...props }: ButtonAsLinkProps) => {
  const forwardedProps = Struct.omit(props, ...buttonStyles.variantKeys);
  const variantProps = Struct.pick(props, ...buttonStyles.variantKeys);
  return <AriaLink {...forwardedProps} className={composeRenderPropsTV(className, buttonStyles, variantProps)} />;
};
