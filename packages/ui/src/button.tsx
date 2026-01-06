import * as RAC from 'react-aria-components';
import { tv, type VariantProps } from 'tailwind-variants';
import { focusVisibleRingStyles } from './focus-ring';
import { LinkComponent, useLink, UseLinkProps } from './router';
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
        border-violet-700 bg-violet-600 text-white

        hover:border-violet-800 hover:bg-violet-700

        disabled:border-violet-400 disabled:bg-violet-400

        pressed:border-violet-900 pressed:bg-violet-800
      `,

      secondary: tw`
        border-slate-200 bg-white text-slate-800

        hover:border-slate-200 hover:bg-slate-100

        pressed:border-slate-300 pressed:bg-white
      `,

      danger: tw`
        border-red-700 bg-red-600 text-white

        hover:border-red-800 hover:bg-red-700

        pressed:border-red-900 pressed:bg-red-800
      `,

      ghost: tw`text-slate-800 hover:bg-slate-100 pressed:bg-slate-200`,

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

export const ButtonAsLink: LinkComponent<ButtonAsLinkProps> = (props) => {
  const { onAction, ...linkProps } = useLink(props as UseLinkProps);
  return <RAC.Link {...props} {...linkProps} className={composeStyleProps(props, buttonStyles)} onPress={onAction} />;
};
