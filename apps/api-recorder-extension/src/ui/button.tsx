import * as RAC from 'react-aria-components';
import { tv, type VariantProps } from 'tailwind-variants';
import { focusVisibleRingStyles } from '@the-dev-tools/ui/focus-ring';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { composeStyleProps } from '@the-dev-tools/ui/utils';

// TODO: remove once extension design is unified with the SaaS

export const buttonStyles = tv({
  extend: focusVisibleRingStyles,
  base: tw`
    flex cursor-pointer items-center justify-center gap-1.5 rounded-lg px-3.5 py-2.5 text-base/5 font-semibold
    whitespace-nowrap select-none
  `,
  variants: {
    variant: {
      primary: tw`bg-indigo-600 text-white hover:bg-indigo-700`,
      'secondary color': tw`border border-indigo-200 bg-indigo-50 text-indigo-700 hover:bg-indigo-100`,
      'secondary gray': tw`border border-slate-200 bg-white text-black hover:bg-slate-100`,
    },
  },
  defaultVariants: {
    variant: 'primary',
  },
});

export interface ButtonProps extends RAC.ButtonProps, VariantProps<typeof buttonStyles> {}

// `composeStyleProps` forwards the variant props to the styles
export const Button = (props: ButtonProps) => (
  <RAC.Button {...props} className={composeStyleProps(props, buttonStyles)} />
);
