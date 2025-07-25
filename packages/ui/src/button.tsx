import { pipe, Record, Struct } from 'effect';
import {
  Button as AriaButton,
  type ButtonProps as AriaButtonProps,
  Link as AriaLink,
  type LinkProps as AriaLinkProps,
} from 'react-aria-components';
import { tv, type VariantProps } from 'tailwind-variants';
import { isFocusVisibleRingRenderPropKeys, isFocusVisibleRingStyles } from './focus-ring';
import { LinkComponent, useLink, UseLinkProps } from './router';
import { tw } from './tailwind-literal';
import { composeRenderPropsTV } from './utils';

export const buttonStyles = tv({
  extend: isFocusVisibleRingStyles,
  base: tw`
    flex cursor-pointer items-center justify-center gap-1 rounded-md border border-transparent bg-transparent px-4
    py-1.5 text-sm leading-5 font-medium tracking-tight select-none
  `,
  variants: {
    ...isFocusVisibleRingStyles.variants,
    isDisabled: { true: tw`cursor-not-allowed` },
    isHovered: { true: null },
    isPressed: { true: null },
    variant: {
      ghost: tw`text-slate-800`,
      'ghost dark': tw`text-white`,
      primary: tw`border-violet-700 bg-violet-600 text-white`,
      secondary: tw`border-slate-200 bg-white text-slate-800`,
    },
  },
  defaultVariants: {
    variant: 'secondary',
  },
  compoundVariants: [
    { className: tw`border-violet-800 bg-violet-700`, isHovered: true, variant: 'primary' },
    { className: tw`border-violet-900 bg-violet-800`, isPressed: true, variant: 'primary' },
    { className: tw`border-violet-400 bg-violet-400`, isDisabled: true, variant: 'primary' },

    { className: tw`border-slate-200 bg-slate-100`, isHovered: true, variant: 'secondary' },
    { className: tw`border-slate-300 bg-white`, isPressed: true, variant: 'secondary' },

    { className: tw`bg-slate-100`, isHovered: true, variant: 'ghost' },
    { className: tw`bg-slate-200`, isPressed: true, variant: 'ghost' },

    { className: tw`bg-slate-600`, isHovered: true, variant: 'ghost dark' },
    { className: tw`bg-slate-700`, isPressed: true, variant: 'ghost dark' },
  ],
});

const renderPropKeys = [...isFocusVisibleRingRenderPropKeys, 'isHovered', 'isPressed', 'isDisabled'] as const;
export const buttonVariantKeys = pipe(Struct.omit(buttonStyles.variants, ...renderPropKeys), Record.keys);

// Button

export interface ButtonProps extends AriaButtonProps, VariantProps<typeof buttonStyles> {}

export const Button = ({ className, ...props }: ButtonProps) => {
  const forwardedProps = Struct.omit(props, ...buttonVariantKeys);
  const variantProps = Struct.pick(props, ...buttonVariantKeys);
  return <AriaButton {...forwardedProps} className={composeRenderPropsTV(className, buttonStyles, variantProps)} />;
};

// Button as link

export interface ButtonAsLinkProps extends AriaLinkProps, VariantProps<typeof buttonStyles> {}

export const ButtonAsLink: LinkComponent<ButtonAsLinkProps> = ({ className, ...props }) => {
  const forwardedProps = Struct.omit(props as ButtonAsLinkProps, ...buttonVariantKeys);
  const variantProps = Struct.pick(props as ButtonAsLinkProps, ...buttonVariantKeys);
  const { onAction, ...linkProps } = useLink(forwardedProps as UseLinkProps);

  return (
    <AriaLink
      {...forwardedProps}
      {...linkProps}
      className={composeRenderPropsTV(className, buttonStyles, variantProps)}
      onPress={onAction}
    />
  );
};
