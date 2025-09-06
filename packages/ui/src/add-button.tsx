import * as RAC from 'react-aria-components';
import { FiPlus } from 'react-icons/fi';
import { tv, VariantProps } from 'tailwind-variants';
import { focusVisibleRingStyles } from './focus-ring';
import { tw } from './tailwind-literal';
import { composeStyleProps } from './utils';

export const addButtonStyles = tv({
  extend: focusVisibleRingStyles,
  base: tw`flex size-5 items-center justify-center rounded-full border font-semibold select-none`,
  variants: {
    variant: {
      dark: tw`
        border-slate-300 text-slate-500

        hover:border-slate-500 hover:text-slate-600

        pressed:border-slate-800 pressed:text-slate-900
      `,
      light: tw`border-white/20 text-white hover:border-white/40 pressed:border-white`,
    },
  },
  defaultVariants: {
    variant: 'dark',
  },
});

export interface AddButtonProps extends Omit<RAC.ButtonProps, 'children'>, VariantProps<typeof addButtonStyles> {
  /** Text to show in the tooltip. If not provided, no tooltip will be shown. */
  tooltipText?: string;
}

export const AddButton = ({ tooltipText, ...props }: AddButtonProps) => {
  let button = (
    <RAC.Button {...props} className={composeStyleProps(props, addButtonStyles)}>
      <FiPlus className={tw`size-4 stroke-[1.2px]`} />
    </RAC.Button>
  );

  // TODO: separate tooltip component
  button = (
    <RAC.TooltipTrigger delay={750}>
      {button}
      <RAC.Tooltip className={tw`rounded-md bg-slate-800 px-2 py-1 text-xs text-white`}>{tooltipText}</RAC.Tooltip>
    </RAC.TooltipTrigger>
  );

  return button;
};
