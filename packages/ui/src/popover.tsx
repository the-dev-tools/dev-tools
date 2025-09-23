import * as RAC from 'react-aria-components';
import { tw } from './tailwind-literal';
import { composeTailwindRenderProps } from './utils';

export interface PopoverProps extends RAC.PopoverProps {}

export const Popover = ({ className, ...props }: PopoverProps) => (
  <RAC.Popover
    {...props}
    className={composeTailwindRenderProps(
      className,
      tw`pointer-events-none flex min-w-(--trigger-width) flex-col placement-top:flex-col-reverse`,
    )}
  />
);
