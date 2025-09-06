import * as RAC from 'react-aria-components';
import { tv, type VariantProps } from 'tailwind-variants';
import { focusVisibleRingStyles } from '@the-dev-tools/ui/focus-ring';
import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { composeStyleRenderProps } from '@the-dev-tools/ui/utils';

// TODO: remove once extension design is unified with the SaaS

export const buttonStyles = tv({
  extend: focusVisibleRingStyles,
  base: tw`
    flex cursor-pointer items-center justify-center gap-1.5 rounded-lg px-4 py-3 text-base leading-5 font-semibold
    select-none

    hover:bg-neutral-400
  `,
  variants: {
    variant: {
      primary: tw`bg-indigo-600 text-white`,
      'secondary color': tw`border border-indigo-200 bg-indigo-50 text-indigo-700`,
      'secondary gray': tw`border border-slate-200 bg-white text-black`,
    },
  },
  defaultVariants: {
    variant: 'primary',
  },
});

export interface ButtonProps extends RAC.ButtonProps, VariantProps<typeof buttonStyles> {}

export const Button = ({ className, ...props }: ButtonProps) => {
  return <RAC.Button {...props} className={composeStyleRenderProps(className, buttonStyles)} />;
};
