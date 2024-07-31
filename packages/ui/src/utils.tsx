import { composeRenderProps } from 'react-aria-components';

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
