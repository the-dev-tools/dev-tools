import { pipe, Record, Struct } from 'effect';
import { Button as AriaButton, ButtonProps as AriaButtonProps, Tooltip, TooltipTrigger } from 'react-aria-components';
import { FiPlus } from 'react-icons/fi';
import { tv, VariantProps } from 'tailwind-variants';

import { isFocusVisibleRingStyles } from './focus-ring';
import { tw } from './tailwind-literal';
import { composeRenderPropsTV } from './utils';

export const addButtonStyles = tv({
  extend: isFocusVisibleRingStyles,
  base: tw`flex size-5 items-center justify-center rounded-full border font-semibold select-none`,
  variants: {
    ...isFocusVisibleRingStyles.variants,
    isHovered: { false: null },
    isPressed: { false: null },
    variant: {
      dark: tw`border-slate-300 text-slate-500`,
      light: tw`border-white/20 text-white`,
    },
  },
  defaultVariants: {
    variant: 'dark',
  },
  compoundVariants: [
    { className: tw`border-slate-500 text-slate-600`, isHovered: true, variant: 'dark' },
    { className: tw`border-slate-800 text-slate-900`, isPressed: true, variant: 'dark' },

    { className: tw`border-white/40`, isHovered: true, variant: 'light' },
    { className: tw`border-white`, isPressed: true, variant: 'light' },
  ],
});

export const addButtonVariantKeys = pipe(
  Struct.omit(addButtonStyles.variants, ...isFocusVisibleRingStyles.variantKeys, 'isHovered', 'isPressed'),
  Record.keys,
);

export interface AddButtonVariantProps
  extends Pick<VariantProps<typeof addButtonStyles>, (typeof addButtonVariantKeys)[number]> {}

export interface AddButtonProps extends AddButtonVariantProps, Omit<AriaButtonProps, 'children'> {
  /** Text to show in the tooltip. If not provided, no tooltip will be shown. */
  tooltipText?: string;
}

export const AddButton = ({ className, tooltipText, ...props }: AddButtonProps) => {
  const forwardedProps = Struct.omit(props, ...addButtonVariantKeys);
  const variantProps = Struct.pick(props, ...addButtonVariantKeys);

  const button = (
    <AriaButton {...forwardedProps} className={composeRenderPropsTV(className, addButtonStyles, variantProps)}>
      <FiPlus className='size-4 stroke-[1.2px]' />
    </AriaButton>
  );

  if (tooltipText) {
    return (
      <TooltipTrigger delay={750}>
        {button}
        <Tooltip className={tw`rounded-md bg-slate-800 px-2 py-1 text-xs text-white`}>{tooltipText}</Tooltip>
      </TooltipTrigger>
    );
  }

  return button;
};
