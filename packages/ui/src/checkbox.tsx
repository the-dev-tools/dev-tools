import { Struct } from 'effect';
import { RefAttributes } from 'react';
import { mergeProps } from 'react-aria';
import * as RAC from 'react-aria-components';
import { FieldPath, FieldValues, useController, UseControllerProps } from 'react-hook-form';
import { tv, VariantProps } from 'tailwind-variants';
import { focusVisibleRingStyles } from './focus-ring';
import { controllerPropKeys, ControllerPropKeys } from './react-hook-form';
import { tw } from './tailwind-literal';
import { composeStyleRenderProps } from './utils';

const checkboxStyles = tv({
  slots: {
    base: tw`group/checkbox flex items-center gap-2`,

    box: [
      focusVisibleRingStyles(),
      tw`
        flex size-4 flex-none cursor-pointer items-center justify-center rounded-sm border border-slate-200 bg-white
        p-0.5 text-white

        group-selected/checkbox:border-violet-600 group-selected/checkbox:bg-violet-600
      `,
    ],
  },
  variants: {
    isTableCell: { true: { base: tw`justify-self-center p-1` } },
  },
});

export interface CheckboxProps
  extends RAC.CheckboxProps,
    RefAttributes<HTMLLabelElement>,
    VariantProps<typeof checkboxStyles> {}

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

// RHF wrapper

export interface CheckboxRHFProps<
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
> extends Omit<CheckboxProps, ControllerPropKeys>,
    UseControllerProps<TFieldValues, TName> {}

export const CheckboxRHF = <
  TFieldValues extends FieldValues = FieldValues,
  TName extends FieldPath<TFieldValues> = FieldPath<TFieldValues>,
>(
  props: CheckboxRHFProps<TFieldValues, TName>,
) => {
  const forwardedProps = Struct.omit(props, ...controllerPropKeys);
  const controllerProps = Struct.pick(props, ...controllerPropKeys);

  const {
    field: { ref, ...field },
    fieldState,
  } = useController({ defaultValue: false as never, ...controllerProps });

  const fieldProps: CheckboxProps = {
    isDisabled: field.disabled ?? false,
    isInvalid: fieldState.invalid,
    isSelected: field.value,
    name: field.name,
    onBlur: field.onBlur,
    onChange: field.onChange,
    validationBehavior: 'aria',
  };

  return <Checkbox {...mergeProps(fieldProps, forwardedProps)} ref={ref} />;
};
