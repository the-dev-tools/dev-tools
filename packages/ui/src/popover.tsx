import * as RAC from 'react-aria-components';
import { tw } from './tailwind-literal';
import { composeTailwindRenderProps } from './utils';

export interface PopoverProps extends RAC.PopoverProps {}

export const Popover = ({ className, offset = 6, ...props }: PopoverProps) => (
  <RAC.Popover
    {...props}
    className={composeTailwindRenderProps(
      className,
      tw`
        pointer-events-none flex min-w-(--trigger-width) origin-(--trigger-anchor-point) flex-col

        motion-reduce:animate-none

        placement-top:flex-col-reverse

        entering:animate-in entering:duration-150 entering:ease-out entering:fade-in-0 entering:zoom-in-[0.97]

        placement-top:entering:slide-in-from-bottom-1

        placement-bottom:entering:slide-in-from-top-1

        exiting:animate-out exiting:duration-100 exiting:ease-in exiting:fade-out-0 exiting:zoom-out-[0.97]
      `,
    )}
    offset={offset}
  />
);
