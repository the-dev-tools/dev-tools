import { pipe, Record, Struct } from 'effect';
import { ComponentProps } from 'react';
import {
  Header as AriaHeader,
  ListBox as AriaListBox,
  ListBoxItem as AriaListBoxItem,
  ListBoxItemProps as AriaListBoxItemProps,
  ListBoxProps as AriaListBoxProps,
  composeRenderProps,
} from 'react-aria-components';
import { FiCheckCircle } from 'react-icons/fi';
import { twJoin, twMerge } from 'tailwind-merge';
import { tv, VariantProps } from 'tailwind-variants';

import { isFocusVisibleRingStyles } from './focus-ring';
import { tw } from './tailwind-literal';
import { ariaTextValue, composeRenderPropsTV } from './utils';

// Root

export const listBoxStyles = tv({
  base: tw`overflow-auto rounded-lg border border-slate-200 bg-white py-0.5 shadow-md outline-none`,
});

export interface ListBoxProps<T> extends Omit<AriaListBoxProps<T>, 'layout' | 'orientation'> {}

export const ListBox = <T extends object>({ className, ...props }: ListBoxProps<T>) => (
  <AriaListBox className={composeRenderPropsTV(className, listBoxStyles)} {...props} />
);

// Item

export const listBoxItemStyles = tv({
  extend: isFocusVisibleRingStyles,
  base: tw`group/listbox flex cursor-pointer select-none items-center gap-2.5 px-3 py-1.5 text-xs font-medium leading-4 tracking-tight -outline-offset-4`,
  variants: {
    ...isFocusVisibleRingStyles.variants,
    variant: {
      default: tw`text-slate-800`,
      danger: tw`text-rose-700`,
      accent: tw`text-violet-600`,
    },
    isHovered: { false: null },
    isPressed: { false: null },
    isSelected: { false: null },
  },
  compoundVariants: [
    { isHovered: true, variant: 'default', className: tw`bg-slate-100` },
    { isHovered: true, variant: 'danger', className: tw`bg-rose-100` },
    { isHovered: true, variant: 'accent', className: tw`bg-violet-100` },

    { isPressed: true, variant: 'default', className: tw`bg-slate-200` },
    { isPressed: true, variant: 'danger', className: tw`bg-rose-200` },
    { isPressed: true, variant: 'accent', className: tw`bg-violet-200` },

    { isSelected: true, variant: 'default', className: tw`bg-slate-200` },
    { isSelected: true, variant: 'danger', className: tw`bg-rose-200` },
    { isSelected: true, variant: 'accent', className: tw`bg-violet-200` },
  ],
  defaultVariants: {
    variant: 'default',
  },
});

export const listBoxItemVariantKeys = pipe(
  Struct.omit(
    listBoxItemStyles.variants,
    ...isFocusVisibleRingStyles.variantKeys,
    'isHovered',
    'isPressed',
    'isSelected',
  ),
  Record.keys,
);

export interface ListBoxItemVariants
  extends Pick<VariantProps<typeof listBoxItemStyles>, (typeof listBoxItemVariantKeys)[number]> {}

export interface ListBoxItemProps extends AriaListBoxItemProps, ListBoxItemVariants {
  showSelectIndicator?: boolean;
}

export const ListBoxItem = ({
  className,
  children,
  textValue,
  showSelectIndicator = true,
  ...props
}: ListBoxItemProps) => {
  const forwardedProps = Struct.omit(props, ...listBoxItemVariantKeys);
  const variantProps = Struct.pick(props, ...listBoxItemVariantKeys);

  return (
    <AriaListBoxItem
      className={composeRenderPropsTV(className, listBoxItemStyles, variantProps)}
      {...ariaTextValue(textValue, children)}
      {...forwardedProps}
    >
      {composeRenderProps(children, (children, { isSelected, selectionMode }) => {
        const selectIndicatorActive = showSelectIndicator && selectionMode !== 'none' && !props.onAction;

        return (
          <>
            {children}
            {selectIndicatorActive && (
              <div className={tw`hidden group-[&[role="option"]]/listbox:contents`}>
                <div className={tw`flex-1`} />
                <FiCheckCircle
                  className={twJoin(
                    tw`size-3.5 stroke-[1.2px] text-green-600 transition-opacity`,
                    isSelected ? tw`opacity-100` : tw`opacity-0`,
                  )}
                />
              </div>
            )}
          </>
        );
      })}
    </AriaListBoxItem>
  );
};

// Header

export interface ListBoxHeaderProps extends ComponentProps<'div'> {}

export const ListBoxHeader = ({ className, ...props }: ListBoxHeaderProps) => (
  <AriaHeader
    {...props}
    className={twMerge(
      tw`select-none px-3 pb-0.5 pt-2 text-xs font-semibold leading-5 tracking-tight text-slate-500`,
      className,
    )}
  />
);
