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
  base: tw`
    pointer-events-auto overflow-auto rounded-lg border border-neutral bg-neutral-lowest py-0.5 shadow-md outline-hidden
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
      accent: tw`
        text-accent

        hover:bg-accent-lowest hover:text-on-accent

        pressed:bg-accent-low pressed:text-on-accent

        selected:bg-accent-low selected:text-on-accent
      `,
      danger: tw`
        text-danger

        hover:bg-danger-lowest hover:text-on-danger

        pressed:bg-danger-low pressed:text-on-danger

        selected:bg-danger-low selected:text-on-danger
      `,
      default: tw`text-on-neutral hover:bg-neutral-low pressed:bg-neutral selected:bg-neutral`,
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
      tw`px-3 pt-2 pb-0.5 text-xs leading-5 font-semibold tracking-tight text-on-neutral-low select-none`,
      className,
    )}
  />
);
