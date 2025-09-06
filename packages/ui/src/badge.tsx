import { tv, VariantProps } from 'tailwind-variants';
import { tw } from './tailwind-literal';

export const badgeStyles = tv({
  base: tw`inline-flex shrink-0 items-center justify-center rounded-md text-xs leading-4 font-semibold`,
  variants: {
    color: {
      amber: tw`border-amber-200 bg-amber-100 text-amber-600`,
      blue: tw`border-blue-200 bg-blue-100 text-blue-600`,
      fuchsia: tw`border-fuchsia-200 bg-fuchsia-100 text-fuchsia-600`,
      green: tw`border-green-200 bg-green-100 text-green-600`,
      purple: tw`border-purple-200 bg-purple-100 text-purple-600`,
      rose: tw`border-rose-200 bg-rose-100 text-rose-600`,
      sky: tw`border-sky-200 bg-sky-100 text-sky-600`,
      slate: tw`border-slate-200 bg-slate-100 text-slate-600`,
    },
    size: {
      default: tw`px-1 py-0.5`,
      lg: tw`p-1`,
    },
  },
  defaultVariants: {
    color: 'slate',
    size: 'default',
  },
});

export interface BadgeProps
  extends Omit<React.ComponentPropsWithoutRef<'div'>, 'color'>,
    VariantProps<typeof badgeStyles> {}

export const Badge = (props: BadgeProps) => <div {...props} className={badgeStyles(props)} />;
