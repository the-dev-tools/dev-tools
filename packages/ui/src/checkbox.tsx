import { RefAttributes } from 'react';
import * as RAC from 'react-aria-components';
import { tv, VariantProps } from 'tailwind-variants';
import { focusVisibleRingStyles } from './focus-ring';
import { tw } from './tailwind-literal';
import { composeStyleRenderProps } from './utils';

const checkboxStyles = tv({
  slots: {
    base: tw`group/checkbox flex items-center gap-2`,

    box: [
      focusVisibleRingStyles(),
      tw`
        flex size-4 flex-none cursor-pointer items-center justify-center rounded-sm border border-border bg-surface
        p-0.5 text-fg-invert

        group-selected/checkbox:border-accent group-selected/checkbox:bg-accent
      `,
    ],
  },
  variants: {
    isTableCell: { true: { base: tw`justify-self-center p-1` } },
  },
});

export interface CheckboxProps
  extends RAC.CheckboxProps, RefAttributes<HTMLLabelElement>, VariantProps<typeof checkboxStyles> {}

export const Checkbox = ({ children, className, ...props }: CheckboxProps) => {
  const styles = checkboxStyles(props);

  return (
    <RAC.Checkbox {...props} className={composeStyleRenderProps(className, styles.base)}>
      {RAC.composeRenderProps(children, (children, renderProps) => (
        <>
          <div className={styles.box()}>
            {renderProps.isSelected && <SelectedIcon />}
            {renderProps.isIndeterminate && <IndeterminateIcon />}
          </div>

          {children}
        </>
      ))}
    </RAC.Checkbox>
  );
};

const SelectedIcon = () => (
  <svg fill='none' height='1em' viewBox='0 0 10 8' width='1em' xmlns='http://www.w3.org/2000/svg'>
    <path
      d='m.833 4.183 2.778 3.15L9.167 1.5'
      stroke='currentColor'
      strokeLinecap='round'
      strokeLinejoin='round'
      strokeWidth={1.2}
    />
  </svg>
);

const IndeterminateIcon = () => (
  <svg fill='none' height='1em' viewBox='0 0 10 2' width='1em' xmlns='http://www.w3.org/2000/svg'>
    <path d='M1 1h8.315' stroke='currentColor' strokeLinecap='round' strokeLinejoin='round' strokeWidth={1.5} />
  </svg>
);
