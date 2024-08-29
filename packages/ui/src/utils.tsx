import { composeRenderProps } from 'react-aria-components';
import { twMerge } from 'tailwind-merge';

export const composeRenderPropsTV = <T, K>(
  className: string | ((renderProps: T) => string) | undefined,
  tv: (variant: T & { className: string | undefined }) => string,
  props?: K,
) =>
  composeRenderProps(className, (className, renderProps) =>
    tv({
      ...(props ?? {}),
      ...renderProps,
      className,
    }),
  );

export const composeRenderPropsTW = <T,>(className: string | ((renderProps: T) => string) | undefined, tw: string) =>
  composeRenderProps(className, (className) => twMerge(tw, className));
