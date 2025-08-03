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
  return { ...(textValue_ && { textValue: textValue_ }) };
};

export const useEscapePortal = <T extends HTMLElement = HTMLDivElement>(
  containerRef: RefObject<HTMLDivElement | null>,
) => {
  const ref = useRef<T>(null);

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

export const formatSize = (bytes: number) => {
  const scale = bytes == 0 ? 0 : Math.floor(Math.log(bytes) / Math.log(1024));
  const size = (bytes / Math.pow(1024, scale)).toFixed(2);
  const name = ['B', 'KiB', 'MiB', 'GiB', 'TiB'][scale];
  return `${size} ${name}`;
};

interface SaveFileProps {
  blobParts: BlobPart[];
  name?: string;
  options?: BlobPropertyBag;
}

export const saveFile = ({ blobParts, name, options }: SaveFileProps) => {
  const link = document.createElement('a');
  const file = new Blob(blobParts, options);
  link.href = URL.createObjectURL(file);
  if (name) link.download = name;
  link.click();
  URL.revokeObjectURL(link.href);
};
