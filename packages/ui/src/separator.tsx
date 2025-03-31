import { Separator as AriaSeparator, SeparatorProps as AriaSeparatorProps } from 'react-aria-components';
import { tv } from 'tailwind-variants';

import { tw } from './tailwind-literal';

const styles = tv({
  base: tw`bg-slate-200`,
  defaultVariants: {
    orientation: 'horizontal',
  },
  variants: {
    orientation: {
      horizontal: tw`h-px w-full`,
      vertical: tw`w-px`,
    },
  },
});

export const Separator = ({ className, ...props }: AriaSeparatorProps) => (
  <AriaSeparator {...props} className={styles({ className, orientation: props.orientation })} />
);
