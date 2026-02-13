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
        border-accent-high bg-accent text-on-accent

        hover:border-accent-higher hover:bg-accent-high

        disabled:border-accent-low disabled:bg-accent-low

        pressed:border-accent-highest pressed:bg-accent-higher
      `,

      secondary: tw`
        border-neutral bg-neutral-lowest text-on-neutral

        hover:bg-neutral-low

        pressed:border-neutral-high pressed:bg-neutral
      `,

      danger: tw`
        border-danger bg-danger-low text-on-danger

        hover:border-danger-high hover:bg-danger

        pressed:border-danger-higher pressed:bg-danger-high
      `,

      ghost: tw`text-on-neutral hover:bg-neutral-low pressed:bg-neutral`,

      'ghost dark': tw`text-on-inverse hover:bg-inverse-lower pressed:bg-inverse-low`,
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
