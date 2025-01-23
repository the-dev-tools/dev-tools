import { ReactNode, RefObject, useCallback, useRef } from 'react';
import { composeRenderProps } from 'react-aria-components';
import { createPortal } from 'react-dom';
import { twMerge } from 'tailwind-merge';

export const composeRenderPropsTV = <T, K>(
  className: string | ((renderProps: T) => string) | undefined,
  tv: (variant: T & K) => string,
  props: K = {} as K,
) =>
  composeRenderProps(className, (className, renderProps) =>
    tv({
      ...props,
      ...renderProps,
      className,
    }),
  );

export const composeRenderPropsTW = <T,>(className: string | ((renderProps: T) => string) | undefined, tw: string) =>
  composeRenderProps(className, (className) => twMerge(tw, className));

export const ariaTextValue = (textValue?: string, children?: unknown) => {
  const textValue_ = textValue ?? (typeof children === 'string' ? children : undefined);
  return textValue_ === undefined ? {} : { textValue: textValue_ };
};

export const useEscapePortal = (containerRef: RefObject<HTMLDivElement | null>) => {
  const ref = useRef<HTMLDivElement>(null);

  const render = useCallback(
    (children: ReactNode) => {
      if (!containerRef.current || !ref.current) return;

      const container = containerRef.current.getBoundingClientRect();
      const target = ref.current.getBoundingClientRect();

      const style = {
        left: target.left - container.left,
        top: target.top - container.top,
        width: target.width,
        height: target.height,
      };

      return createPortal(
        <div className='absolute size-full' style={style}>
          {children}
        </div>,
        containerRef.current,
      );
    },
    [containerRef],
  );

  return { ref, render };
};
