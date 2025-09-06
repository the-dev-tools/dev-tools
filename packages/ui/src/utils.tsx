import { ReactNode, RefObject, useCallback, useRef } from 'react';
import { composeRenderProps, StyleRenderProps } from 'react-aria-components';
import { createPortal } from 'react-dom';
import { twMerge } from 'tailwind-merge';
import { ClassProp } from 'tailwind-variants';
import { tw } from './tailwind-literal';

export const composeTailwindRenderProps = <TRenderProps,>(
  className: StyleRenderProps<TRenderProps>['className'],
  ...tw: string[]
) => composeRenderProps(className, (className) => twMerge(...tw, className));

export const composeStyleRenderProps = <TRenderProps, TVariantProps, TExtraProps>(
  className: StyleRenderProps<TRenderProps>['className'],
  tv: (props: ClassProp & TExtraProps & TVariantProps) => string,
  extraProps?: TExtraProps,
) => composeRenderProps(className, (className, renderProps) => tv({ ...extraProps, ...renderProps, className }));

export const composeStyleProps = <TRenderProps, TVariantProps>(
  props: StyleRenderProps<TRenderProps> & TVariantProps,
  tv: ((props: ClassProp & TVariantProps) => string) & { variantKeys: (keyof TVariantProps)[] },
) => composeRenderProps(props.className, (className, renderProps) => tv({ ...props, ...renderProps, className }));

export const composeTextValueProps = (props: { children?: unknown; textValue?: string }) => {
  const textValue = props.textValue ?? (typeof props.children === 'string' ? props.children : undefined);
  return { ...(textValue && { textValue }) };
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
  blobParts: BlobPart[] | Uint8Array[];
  name?: string;
  options?: BlobPropertyBag;
}

export const saveFile = ({ blobParts, name, options }: SaveFileProps) => {
  const link = document.createElement('a');
  // TODO: remove casting once fixed upstream https://github.com/DefinitelyTyped/DefinitelyTyped/pull/73414
  const file = new Blob(blobParts as BlobPart[], options);
  link.href = URL.createObjectURL(file);
  if (name) link.download = name;
  link.click();
  URL.revokeObjectURL(link.href);
};
