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
  base: tw`flex cursor-pointer select-none items-center justify-center`,
  variants: {
    kind: {
      default: tw`gap-1.5 rounded-lg px-4 py-3 text-base font-semibold leading-5`,
      placeholder: tw`gap-0.5 rounded border px-3 py-2 text-sm leading-none text-black`,
    },
    variant: {
      primary: tw`bg-indigo-600 text-white`,
      'secondary gray': tw`border border-slate-200 bg-white text-black`,
      'secondary color': tw`border border-indigo-200 bg-indigo-50 text-indigo-700`,
      placeholder: tw`border-black bg-neutral-200`,
      'placeholder ghost': tw`border-transparent bg-transparent p-1`,
    },
    isHovered: { true: null },
  },
  compoundVariants: [
    {
      variant: 'placeholder',
      isHovered: true,
      className: tw`bg-neutral-400`,
    },
  ],
  defaultVariants: {
    kind: 'default',
    variant: 'primary',
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
