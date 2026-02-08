import { HKT } from 'effect';
import { ComponentProps } from 'react';
import * as RAC from 'react-aria-components';
import { twMerge } from 'tailwind-merge';
import { tv, VariantProps } from 'tailwind-variants';
import { focusVisibleRingStyles } from './focus-ring';
import { tw } from './tailwind-literal';
import { composeStyleProps, composeStyleRenderProps } from './utils';
import { createLinkGeneric } from './utils/link';

// Root

export const listBoxStyles = tv({
  base: tw`pointer-events-auto overflow-auto rounded-lg border border-border bg-surface py-0.5 shadow-md outline-hidden`,
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
      accent: tw`text-accent-fg hover:bg-accent-soft pressed:bg-accent-soft-active selected:bg-accent-soft-active`,
      danger: tw`text-danger-soft-fg hover:bg-danger-soft pressed:bg-danger-soft-active selected:bg-danger-soft-active`,
      default: tw`text-fg hover:bg-surface-hover pressed:bg-surface-active selected:bg-surface-active`,
    },
  },
  defaultVariants: {
    variant: 'default',
  },
});

export interface ListBoxItemProps<T = object> extends RAC.ListBoxItemProps<T>, VariantProps<typeof listBoxItemStyles> {}

export const ListBoxItem = <T extends object>({ ...props }: ListBoxItemProps<T>) => (
  <RAC.ListBoxItem {...props} className={composeStyleProps(props, listBoxItemStyles)} />
);

export interface ListBoxItemTypeLambda extends HKT.TypeLambda {
  readonly type: typeof ListBoxItem<this['Target'] extends object ? this['Target'] : never>;
}

export const ListBoxItemRouteLink = createLinkGeneric<ListBoxItemTypeLambda, object>(ListBoxItem);

// Header

export interface ListBoxHeaderProps extends ComponentProps<'div'> {}

export const ListBoxHeader = ({ className, ...props }: ListBoxHeaderProps) => (
  <RAC.Header
    {...props}
    className={twMerge(
      tw`px-3 pt-2 pb-0.5 text-xs leading-5 font-semibold tracking-tight text-fg-muted select-none`,
      className,
    )}
  />
);
