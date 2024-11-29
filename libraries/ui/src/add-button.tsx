import { pipe, Record, Struct } from 'effect';
import { Button as AriaButton, ButtonProps as AriaButtonProps } from 'react-aria-components';
import { FiPlus } from 'react-icons/fi';
import { tv, VariantProps } from 'tailwind-variants';

import { isFocusVisibleRingStyles } from './focus-ring';
import { tw } from './tailwind-literal';
import { composeRenderPropsTV } from './utils';

export const addButtonStyles = tv({
  extend: isFocusVisibleRingStyles,
  base: tw`flex size-5 select-none items-center justify-center rounded-full border font-semibold`,
  variants: {
    ...isFocusVisibleRingStyles.variants,
    variant: {
      dark: tw`border-slate-300 text-slate-500`,
      light: tw`border-white/20 text-white`,
    },
    isHovered: { false: null },
    isPressed: { false: null },
  },
  compoundVariants: [
    { variant: 'dark', isHovered: true, className: tw`border-slate-500 text-slate-600` },
    { variant: 'dark', isPressed: true, className: tw`border-slate-800 text-slate-900` },

    { variant: 'light', isHovered: true, className: tw`border-white/40` },
    { variant: 'light', isPressed: true, className: tw`border-white` },
  ],
  defaultVariants: {
    variant: 'dark',
  },
});

export const addButtonVariantKeys = pipe(
  Struct.omit(addButtonStyles.variants, ...isFocusVisibleRingStyles.variantKeys, 'isHovered', 'isPressed'),
  Record.keys,
);

export interface AddButtonVariantProps
  extends Pick<VariantProps<typeof addButtonStyles>, (typeof addButtonVariantKeys)[number]> {}

export interface AddButtonProps extends Omit<AriaButtonProps, 'children'>, AddButtonVariantProps {}

export const AddButton = ({ className, ...props }: AddButtonProps) => {
  const forwardedProps = Struct.omit(props, ...addButtonVariantKeys);
  const variantProps = Struct.pick(props, ...addButtonVariantKeys);

  return (
    <AriaButton {...forwardedProps} className={composeRenderPropsTV(className, addButtonStyles, variantProps)}>
      <FiPlus className='size-4 stroke-[1.2px]' />
    </AriaButton>
  );
};
