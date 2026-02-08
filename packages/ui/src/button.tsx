import { createLink } from '@tanstack/react-router';
import * as RAC from 'react-aria-components';
import { tv, type VariantProps } from 'tailwind-variants';
import { focusVisibleRingStyles } from './focus-ring';
import { Spinner } from './spinner';
import { tw } from './tailwind-literal';
import { composeStyleProps } from './utils';

export const buttonStyles = tv({
  extend: focusVisibleRingStyles,
  base: tw`
    relative flex cursor-pointer items-center justify-center gap-1 overflow-hidden rounded-md border border-transparent
    bg-transparent px-4 py-1.5 text-sm leading-5 font-medium tracking-tight select-none

    disabled:cursor-not-allowed

    pending:cursor-progress pending:text-transparent!
  `,
  variants: {
    variant: {
      primary: tw`
        border-accent-border bg-accent text-fg-invert

        hover:border-accent-border-hover hover:bg-accent-hover

        disabled:border-accent-disabled disabled:bg-accent-disabled

        pressed:border-accent-border-pressed pressed:bg-accent-pressed
      `,

      secondary: tw`
        border-border bg-surface text-fg

        hover:border-border hover:bg-surface-hover

        pressed:border-border-emphasis pressed:bg-surface
      `,

      danger: tw`
        border-danger-border bg-danger text-fg-invert

        hover:border-danger-pressed hover:bg-danger-hover

        pressed:border-danger-pressed pressed:bg-danger-pressed
      `,

      ghost: tw`text-fg hover:bg-surface-hover pressed:bg-surface-active`,

      'ghost dark': tw`text-white hover:bg-slate-600 pressed:bg-slate-700`,
    },
  },
  defaultVariants: {
    variant: 'secondary',
  },
});

export interface ButtonProps extends RAC.ButtonProps, VariantProps<typeof buttonStyles> {}

export const Button = ({ children, ...props }: ButtonProps) => (
  <RAC.Button {...props} className={composeStyleProps(props, buttonStyles)}>
    {RAC.composeRenderProps(children, (children, { isPending }) => (
      <>
        {children}
        {isPending && <PendingIndicator />}
      </>
    ))}
  </RAC.Button>
);

const PendingIndicator = () => (
  <div className={tw`absolute flex size-full items-center justify-center`}>
    <Spinner />
  </div>
);

// As link

export interface ButtonAsLinkProps extends RAC.LinkProps, VariantProps<typeof buttonStyles> {}

export const ButtonAsLink = (props: ButtonAsLinkProps) => (
  <RAC.Link {...props} className={composeStyleProps(props, buttonStyles)} />
);

export const ButtonAsRouteLink = createLink(ButtonAsLink);
