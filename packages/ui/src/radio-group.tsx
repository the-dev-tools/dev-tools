import * as RAC from 'react-aria-components';
import { tv, VariantProps } from 'tailwind-variants';
import { FieldError, FieldErrorProps, FieldLabel, FieldLabelProps } from './field';
import { focusVisibleRingStyles } from './focus-ring';
import { tw } from './tailwind-literal';
import { composeStyleRenderProps } from './utils';

// Group

export const radioGroupStyles = tv({
  slots: {
    base: tw`flex flex-col gap-2`,
    container: tw`flex`,
  },
  variants: {
    orientation: {
      horizontal: { container: tw`gap-3` },
      vertical: { container: tw`flex-col` },
    },
  },
  defaultVariants: {
    orientation: 'vertical',
  },
});

export interface RadioGroupProps extends RAC.RadioGroupProps, VariantProps<typeof radioGroupStyles> {
  error?: FieldErrorProps['children'];
  label?: FieldLabelProps['children'];
}

export const RadioGroup = ({ children, className, error, label, ...props }: RadioGroupProps) => {
  const styles = radioGroupStyles(props);

  return (
    <RAC.RadioGroup {...props} className={composeStyleRenderProps(className, styles.base)}>
      {RAC.composeRenderProps(children, (children) => (
        <>
          {label && <FieldLabel>{label}</FieldLabel>}
          <div className={styles.container()}>{children}</div>
          <FieldError>{error}</FieldError>
        </>
      ))}
    </RAC.RadioGroup>
  );
};

// Item

export const radioStyles = tv({
  slots: {
    base: tw`
      group/radio flex cursor-pointer items-center gap-1.5 text-md leading-5 font-medium tracking-tight text-fg

      disabled:text-gray-300
    `,

    indicator: [
      focusVisibleRingStyles(),
      tw`
        size-4 rounded-full border border-border bg-surface

        group-invalid/radio:border-danger-fg group-invalid/radio:bg-danger-fg

        group-disabled/radio:border-border group-disabled/radio:bg-border

        group-pressed/radio:not-selected:border-fg-subtle

        group-selected/radio:border-accent group-selected/radio:bg-accent

        group-invalid/radio:pressed:border-danger-pressed
      `,
    ],

    dot: tw`size-full rounded-full border-2 border-surface`,
  },
});

export interface RadioProps extends RAC.RadioProps {}

export const Radio = ({ children, className, ...props }: RadioProps) => {
  const styles = radioStyles(props);

  return (
    <RAC.Radio {...props} className={composeStyleRenderProps(className, styles.base)}>
      {RAC.composeRenderProps(children, (children) => (
        <>
          <div className={styles.indicator()}>
            <div className={styles.dot()} />
          </div>

          {children}
        </>
      ))}
    </RAC.Radio>
  );
};
