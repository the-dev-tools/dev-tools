import { ComponentProps } from 'react';
import * as RAC from 'react-aria-components';
import { twMerge } from 'tailwind-merge';
import { tv, VariantProps } from 'tailwind-variants';
import { focusVisibleRingStyles } from './focus-ring';
import { LinkComponent, useLink, UseLinkProps } from './router';
import { tw } from './tailwind-literal';
import { composeStyleProps, composeStyleRenderProps } from './utils';

// Root

export const listBoxStyles = tv({
  base: tw`
    pointer-events-auto overflow-auto rounded-lg border border-slate-200 bg-white py-0.5 shadow-md outline-hidden
  `,
});

export interface ListBoxProps<T>
  extends Omit<RAC.ListBoxProps<T>, 'layout' | 'orientation'>, VariantProps<typeof listBoxStyles> {}

export const ListBox = <T extends object>({ className, ...props }: ListBoxProps<T>) => (
  <RAC.ListBox className={composeStyleRenderProps(className, listBoxStyles)} {...props} />
);

// Item

export const listBoxItemStyles = tv({
  extend: focusVisibleRingStyles,
  base: tw`
    group/listbox group/list-item flex cursor-pointer items-center gap-2.5 px-3 py-1.5 text-xs leading-4 font-medium
    tracking-tight -outline-offset-4 select-none
  `,
  variants: {
    variant: {
      accent: tw`text-violet-600 hover:bg-violet-100 pressed:bg-violet-200 selected:bg-violet-200`,
      danger: tw`text-rose-700 hover:bg-rose-100 pressed:bg-rose-200 selected:bg-rose-200`,
      default: tw`text-slate-800 hover:bg-slate-100 pressed:bg-slate-200 selected:bg-slate-200`,
    },
  },
  defaultVariants: {
    variant: 'default',
  },
});

export interface ListBoxItemProps extends RAC.ListBoxItemProps, VariantProps<typeof listBoxItemStyles> {}

export const ListBoxItem = ({ ...props }: ListBoxItemProps) => (
  <RAC.ListBoxItem {...props} className={composeStyleProps(props, listBoxItemStyles)} />
);

export const ListBoxItemLink: LinkComponent<ListBoxItemProps> = (props) => {
  const linkProps = useLink(props as UseLinkProps<'div'>);
  return <ListBoxItem {...(props as ListBoxItemProps)} {...linkProps} />;
};

// Header

export interface ListBoxHeaderProps extends ComponentProps<'div'> {}

export const ListBoxHeader = ({ className, ...props }: ListBoxHeaderProps) => (
  <RAC.Header
    {...props}
    className={twMerge(
      tw`px-3 pt-2 pb-0.5 text-xs leading-5 font-semibold tracking-tight text-slate-500 select-none`,
      className,
    )}
  />
);
