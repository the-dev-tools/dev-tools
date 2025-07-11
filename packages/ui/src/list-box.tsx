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
import { LinkComponent, useLink, UseLinkProps } from './router';
import { tw } from './tailwind-literal';
import { ariaTextValue, composeRenderPropsTV } from './utils';

// Root

export const listBoxStyles = tv({
  base: tw`
    pointer-events-auto overflow-auto rounded-lg border border-slate-200 bg-white py-0.5 shadow-md outline-hidden
  `,
});

export interface ListBoxProps<T> extends Omit<AriaListBoxProps<T>, 'layout' | 'orientation'> {}

export const ListBox = <T extends object>({ className, ...props }: ListBoxProps<T>) => (
  <AriaListBox className={composeRenderPropsTV(className, listBoxStyles)} {...props} />
);

// Item

export const listBoxItemStyles = tv({
  extend: isFocusVisibleRingStyles,
  base: tw`
    group/listbox flex cursor-pointer items-center gap-2.5 px-3 py-1.5 text-xs leading-4 font-medium tracking-tight
    -outline-offset-4 select-none
  `,
  variants: {
    ...isFocusVisibleRingStyles.variants,
    isHovered: { false: null },
    isPressed: { false: null },
    isSelected: { false: null },
    variant: {
      accent: tw`text-violet-600`,
      danger: tw`text-rose-700`,
      default: tw`text-slate-800`,
    },
  },
  defaultVariants: {
    variant: 'default',
  },
  compoundVariants: [
    { className: tw`bg-slate-100`, isHovered: true, variant: 'default' },
    { className: tw`bg-rose-100`, isHovered: true, variant: 'danger' },
    { className: tw`bg-violet-100`, isHovered: true, variant: 'accent' },

    { className: tw`bg-slate-200`, isPressed: true, variant: 'default' },
    { className: tw`bg-rose-200`, isPressed: true, variant: 'danger' },
    { className: tw`bg-violet-200`, isPressed: true, variant: 'accent' },

    { className: tw`bg-slate-200`, isSelected: true, variant: 'default' },
    { className: tw`bg-rose-200`, isSelected: true, variant: 'danger' },
    { className: tw`bg-violet-200`, isSelected: true, variant: 'accent' },
  ],
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
  children,
  className,
  showSelectIndicator = true,
  textValue,
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

export const ListBoxItemLink: LinkComponent<ListBoxItemProps> = (props) => {
  const linkProps = useLink(props as UseLinkProps);
  return <ListBoxItem {...(props as ListBoxItemProps)} {...linkProps} />;
};

// Header

export interface ListBoxHeaderProps extends ComponentProps<'div'> {}

export const ListBoxHeader = ({ className, ...props }: ListBoxHeaderProps) => (
  <AriaHeader
    {...props}
    className={twMerge(
      tw`px-3 pt-2 pb-0.5 text-xs leading-5 font-semibold tracking-tight text-slate-500 select-none`,
      className,
    )}
  />
);
