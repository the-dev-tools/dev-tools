import { Struct } from 'effect';
import { Button as AriaButton, type ButtonProps as AriaButtonProps } from 'react-aria-components';
import { tv, type VariantProps } from 'tailwind-variants';

import { focusRingStyles } from '@the-dev-tools/ui/focus-ring';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { composeRenderPropsTV } from '@the-dev-tools/ui/utils';

// TODO: remove once extension design is unified with the SaaS

// Button

export const buttonStyles = tv({
  base: tw`flex cursor-pointer select-none items-center justify-center gap-1.5 rounded-lg px-4 py-3 text-base font-semibold leading-5`,
  compoundVariants: [
    {
      className: tw`bg-neutral-400`,
      isHovered: true,
    },
  ],
  defaultVariants: {
    variant: 'primary',
  },
  extend: focusRingStyles,
  variants: {
    isHovered: { true: null },
    variant: {
      primary: tw`bg-indigo-600 text-white`,
      'secondary color': tw`border border-indigo-200 bg-indigo-50 text-indigo-700`,
      'secondary gray': tw`border border-slate-200 bg-white text-black`,
    },
  },
});

export interface ButtonProps extends AriaButtonProps, VariantProps<typeof buttonStyles> {}

export const Button = ({ className, ...props }: ButtonProps) => {
  const forwardedProps = Struct.omit(props, ...buttonStyles.variantKeys);
  const variantProps = Struct.pick(props, ...buttonStyles.variantKeys);
  return <AriaButton {...forwardedProps} className={composeRenderPropsTV(className, buttonStyles, variantProps)} />;
};
