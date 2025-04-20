import { useState } from 'react';
import { mergeProps } from 'react-aria';
import {
  PanelResizeHandle as UpstreamPanelResizeHandle,
  type PanelResizeHandleProps as UpstreamPanelResizeHandleProps,
} from 'react-resizable-panels';
import { tv, VariantProps } from 'tailwind-variants';

import { focusRingStyles } from './focus-ring';
import { tw } from './tailwind-literal';

// Resize handle

export const panelResizeHandleStyles = tv({
  extend: focusRingStyles,
  base: tw`bg-slate-200`,
  variants: {
    direction: {
      horizontal: tw`h-full w-px cursor-col-resize`,
      vertical: tw`h-px w-full cursor-row-resize`,
    },
  },
});

export interface PanelResizeHandleProps
  extends Omit<VariantProps<typeof panelResizeHandleStyles>, 'direction'>,
    Required<Pick<VariantProps<typeof panelResizeHandleStyles>, 'direction'>>,
    UpstreamPanelResizeHandleProps {}

export const PanelResizeHandle = ({ className, direction, ...props }: PanelResizeHandleProps) => {
  const [isFocused, setIsFocused] = useState(false);
  const forwardedProps = mergeProps(props, {
    onBlur: () => void setIsFocused(false),
    onFocus: () => void setIsFocused(true),
  });
  return (
    <UpstreamPanelResizeHandle
      {...forwardedProps}
      className={panelResizeHandleStyles({ className, direction, isFocused })}
    />
  );
};
