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
        border-primary bg-primary text-primary-foreground

        hover:border-primary/80 hover:bg-primary/90

        disabled:border-primary/50 disabled:bg-primary/50

        pressed:border-primary/70 pressed:bg-primary/80
      `,

      secondary: tw`
        border-border bg-background text-foreground

        hover:border-border hover:bg-secondary

        pressed:border-input pressed:bg-background
      `,

      danger: tw`
        border-destructive bg-destructive text-primary-foreground

        hover:border-destructive/80 hover:bg-destructive/90

        pressed:border-destructive/80 pressed:bg-destructive/80
      `,

      ghost: tw`text-foreground hover:bg-secondary pressed:bg-accent`,

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
