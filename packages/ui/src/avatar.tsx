import { Struct } from 'effect';
import { ComponentProps } from 'react';
import { tv, VariantProps } from 'tailwind-variants';

import { tw } from './tailwind-literal';

export const avatarStyles = tv({
  base: tw`flex size-5 select-none items-center justify-center rounded-full border text-[0.625rem] font-semibold`,
  variants: {
    variant: {
      neutral: tw`border-slate-200 bg-white text-slate-800`,
      amber: tw`border-amber-500 bg-amber-100 text-amber-600`,
      blue: tw`border-blue-400 bg-blue-100 text-blue-600`,
      lime: tw`border-lime-500 bg-lime-200 text-lime-600`,
      pink: tw`border-pink-400 bg-pink-100 text-pink-600`,
      teal: tw`border-teal-400 bg-teal-100 text-teal-600`,
      violet: tw`border-violet-400 bg-violet-200 text-violet-600`,
    },
  },
  defaultVariants: {
    variant: 'neutral',
  },
});

export interface AvatarProps extends ComponentProps<'div'>, VariantProps<typeof avatarStyles> {
  children: string;
}

export const Avatar = ({ children, className, ...props }: AvatarProps) => {
  const forwardedProps = Struct.omit(props, ...avatarStyles.variantKeys);
  const variantProps = Struct.pick(props, ...avatarStyles.variantKeys);
  return (
    <div {...forwardedProps} className={avatarStyles({ ...variantProps, className })}>
      {children[0]?.toUpperCase()}
    </div>
  );
};
