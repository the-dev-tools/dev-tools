import { HKT } from 'effect';
import { ComponentProps } from 'react';
import * as RAC from 'react-aria-components';
import { twMerge } from 'tailwind-merge';
import { tv, VariantProps } from 'tailwind-variants';
import { tw } from './tailwind-literal';
import { composeStyleProps, composeStyleRenderProps } from './utils';
import { createLinkGeneric } from './utils/link';

// Root

export const listBoxStyles = tv({
  base: tw`
    pointer-events-auto max-h-80 min-w-44 overflow-auto rounded-xl border border-neutral bg-neutral-lowest p-1
    shadow-[0_12px_32px_-8px_rgb(0_0_0/0.28),0_2px_6px_-2px_rgb(0_0_0/0.16)] outline-hidden

    dark:bg-neutral-lower dark:shadow-[0_16px_40px_-8px_rgb(0_0_0/0.6),0_2px_8px_-2px_rgb(0_0_0/0.4)]
  `,
});

export interface ListBoxProps<T>
  extends Omit<RAC.ListBoxProps<T>, 'layout' | 'orientation'>, VariantProps<typeof listBoxStyles> {}

export const ListBox = <T extends object>({ className, ...props }: ListBoxProps<T>) => (
  <RAC.ListBox className={composeStyleRenderProps(className, listBoxStyles)} {...props} />
);

// Item

// Items highlight on hover and keyboard focus alike (a menu's focus indicator is the row
// itself); the chosen value is marked by a check in SelectItem, not by a background.
export const listBoxItemStyles = tv({
  base: tw`
    group/listbox group/list-item flex min-h-8 cursor-default items-center gap-2 rounded-md px-2.5 py-1.5 text-md/5
    font-medium tracking-tight outline-hidden transition-colors duration-100 select-none

    disabled:cursor-not-allowed disabled:opacity-45
  `,
  variants: {
    variant: {
      accent: tw`
        text-accent

        hover:bg-accent-lowest

        data-focused:bg-accent-lowest

        pressed:bg-accent-low pressed:text-on-accent
      `,
      danger: tw`
        text-danger

        hover:bg-danger-lowest

        data-focused:bg-danger-lowest

        pressed:bg-danger-low pressed:text-on-danger
      `,
      default: tw`
        text-on-neutral

        hover:bg-neutral-low

        data-focused:bg-neutral-low

        pressed:bg-neutral

        selected:font-semibold
      `,
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
      tw`px-2.5 pt-2 pb-1 text-xs/4 font-semibold tracking-wide text-on-neutral-low uppercase select-none`,
      className,
    )}
  />
);
