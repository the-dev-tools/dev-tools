import * as RRP from 'react-resizable-panels';
import { tv, VariantProps } from 'tailwind-variants';
import { focusVisibleRingStyles } from './focus-ring';
import { tw } from './tailwind-literal';

export const panelResizeHandleStyles = tv({
  extend: focusVisibleRingStyles,
  base: tw`bg-slate-200`,
  variants: {
    direction: {
      horizontal: tw`h-full w-px cursor-col-resize`,
      vertical: tw`h-px w-full cursor-row-resize`,
    },
  },
});

export interface PanelResizeHandleProps extends RRP.SeparatorProps, VariantProps<typeof panelResizeHandleStyles> {}

export const PanelResizeHandle = (props: PanelResizeHandleProps) => (
  <RRP.Separator {...props} className={panelResizeHandleStyles(props)} />
);
