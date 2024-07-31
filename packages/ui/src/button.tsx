import { Button as RACButton, ButtonProps as RACButtonProps } from 'react-aria-components';
import { tv, type VariantProps } from 'tailwind-variants';

import { focusRingStyles } from './focus-ring';
import { tw } from './tailwind-literal';
import { composeRenderPropsTV } from './utils';

export const buttonStyles = tv({
  extend: focusRingStyles,
  base: tw`flex cursor-pointer items-center justify-center gap-1.5 rounded-lg px-4 py-3 text-base font-semibold leading-5`,
  variants: {
    variant: {
      primary: tw`bg-indigo-600 text-white`,
      'secondary gray': tw`border border-slate-200 bg-white text-black`,
      'secondary color': tw`border border-indigo-200 bg-indigo-50 text-indigo-700`,
    },
  },
  defaultVariants: {
    variant: 'primary',
  },
});

export interface ButtonProps extends RACButtonProps, VariantProps<typeof buttonStyles> {}

export const Button = ({ className, variant, ...props }: ButtonProps) => (
  <RACButton {...props} className={composeRenderPropsTV(className, buttonStyles, { variant })} />
);
