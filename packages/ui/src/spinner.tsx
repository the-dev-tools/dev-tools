import { ProgressBar } from 'react-aria-components';
import { tv, VariantProps } from 'tailwind-variants';
import { tw } from './tailwind-literal';

export const spinnerStyles = tv({
  base: tw`animate-spin`,
  variants: {
    size: {
      sm: tw`size-4`,
      md: tw`size-8`,
      lg: tw`size-12`,
      xl: tw`size-16`,
    },
  },
  defaultVariants: { size: 'sm' },
});

export interface SpinnerProps extends VariantProps<typeof spinnerStyles> {
  className?: string;
}

export const Spinner = (props: SpinnerProps) => (
  <ProgressBar aria-label='Loading...' isIndeterminate>
    <svg
      className={spinnerStyles(props)}
      fill='none'
      height='1em'
      viewBox='0 0 60 60'
      width='1em'
      xmlns='http://www.w3.org/2000/svg'
    >
      <clipPath id='spinner'>
        <path d='M55 30c0 13.807-11.193 25-25 25S5 43.807 5 30 16.193 5 30 5s25 11.193 25 25Zm-41.25 0c0 8.975 7.275 16.25 16.25 16.25S46.25 38.975 46.25 30 38.975 13.75 30 13.75 13.75 21.025 13.75 30Z' />
      </clipPath>
      <foreignObject clipPath='url(#spinner)' height='100%' width='100%' x='0' y='0'>
        <div
          className={tw`size-full rounded-full`}
          style={{ backgroundImage: 'conic-gradient(var(--color-surface-active), var(--color-fg-muted))' }}
        />
      </foreignObject>
    </svg>
  </ProgressBar>
);
