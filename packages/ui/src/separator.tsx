import * as RAC from 'react-aria-components';
import { tv, VariantProps } from 'tailwind-variants';
import { tw } from './tailwind-literal';

export const separatorStyles = tv({
  base: tw`bg-border`,
  variants: {
    orientation: {
      horizontal: tw`h-px w-full`,
      vertical: tw`w-px`,
    },
  },
  defaultVariants: {
    orientation: 'horizontal',
  },
});

export interface SeparatorProps extends RAC.SeparatorProps, VariantProps<typeof separatorStyles> {}

export const Separator = (props: SeparatorProps) => <RAC.Separator {...props} className={separatorStyles(props)} />;
