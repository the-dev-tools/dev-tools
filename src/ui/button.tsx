import * as RAC from 'react-aria-components';
import { tv, type VariantProps } from 'tailwind-variants';

import * as Utils from '@/utils';
import { tw } from '@/utils';

import * as FocusRing from './focus-ring';

const styles = tv({
  extend: FocusRing.styles,
  base: tw`cursor-pointer rounded-lg px-4 py-3 text-base font-semibold leading-5`,
  variants: {
    variant: {
      primary: tw`bg-indigo-600 text-white`,
      secondary: tw`border border-slate-200 bg-white text-black`,
    },
  },
  defaultVariants: {
    variant: 'primary',
  },
});

interface Props extends RAC.ButtonProps, VariantProps<typeof styles> {}

export const Main = ({ className, variant, ...props }: Props) => (
  <RAC.Button {...props} className={Utils.Aria.composeRenderPropsTV(className, styles, { variant })} />
);
