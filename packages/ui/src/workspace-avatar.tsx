import { Struct } from 'effect';
import { ComponentProps } from 'react';
import { tv, VariantProps } from 'tailwind-variants';

import { avatarStyles } from './avatar';
import { tw } from './tailwind-literal';

export const workspaceAvatarStyles = tv({
  base: tw`flex size-9 select-none items-center justify-center rounded-md border font-semibold`,
  variants: {
    ...avatarStyles.variants,
  },
  defaultVariants: {
    variant: 'neutral',
  },
});

export interface WorkspaceAvatarProps extends ComponentProps<'div'>, VariantProps<typeof workspaceAvatarStyles> {
  children: string;
}

export const WorkspaceAvatar = ({ children, className, ...props }: WorkspaceAvatarProps) => {
  const forwardedProps = Struct.omit(props, ...workspaceAvatarStyles.variantKeys);
  const variantProps = Struct.pick(props, ...workspaceAvatarStyles.variantKeys);
  return (
    <div {...forwardedProps} className={workspaceAvatarStyles({ ...variantProps, className })}>
      {children[0]?.toUpperCase()}
    </div>
  );
};
