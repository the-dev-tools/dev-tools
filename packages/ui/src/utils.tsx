import { ReactNode, RefObject, useCallback, useRef } from 'react';
import { composeRenderProps } from 'react-aria-components';
import { createPortal } from 'react-dom';
import { twMerge } from 'tailwind-merge';

import { tw } from './tailwind-literal';

export const composeRenderPropsTV = <T, K>(
  className: ((renderProps: T) => string) | string | undefined,
  tv: (variant: K & T) => string,
  props: K = {} as K,
) =>
  composeRenderProps(className, (className, renderProps) =>
    tv({
      ...props,
      ...renderProps,
      className,
    }),
  );

export const composeRenderPropsTW = <T,>(className: ((renderProps: T) => string) | string | undefined, tw: string) =>
  composeRenderProps(className, (className) => twMerge(tw, className));

export const ariaTextValue = (textValue?: string, children?: unknown) => {
  const textValue_ = textValue ?? (typeof children === 'string' ? children : undefined);
  return textValue_ === undefined ? {} : { textValue: textValue_ };
};

export const useEscapePortal = (containerRef: RefObject<HTMLDivElement | null>) => {
  const ref = useRef<HTMLDivElement>(null);

  const render = useCallback(
    (children: ReactNode, zoom = 1) => {
      if (!containerRef.current || !ref.current) return;

      const container = containerRef.current.getBoundingClientRect();
      const target = ref.current.getBoundingClientRect();

      const style = {
        height: target.height / zoom,
        left: (target.left - container.left) / zoom,
        top: (target.top - container.top) / zoom,
        width: target.width / zoom,
      };

      return createPortal(
        <div className={tw`absolute flex size-full items-center`} style={style}>
          {children}
        </div>,
        containerRef.current,
      );
    },
    [containerRef],
  );

  return { ref, render };
};
