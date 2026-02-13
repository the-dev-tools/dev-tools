import { RefAttributes } from 'react';
import * as RAC from 'react-aria-components';
import { FiMinus, FiPlus } from 'react-icons/fi';
import { FieldLabel, FieldLabelProps } from './field';
import { focusVisibleRingStyles } from './focus-ring';
import { tw } from './tailwind-literal';
import { composeTailwindRenderProps } from './utils';

// Number field

export interface NumberFieldProps extends RAC.NumberFieldProps, RefAttributes<HTMLDivElement> {
  groupClassName?: RAC.GroupProps['className'];
  label?: FieldLabelProps['children'];
}

export const NumberField = ({ className = '', groupClassName, label, ...props }: NumberFieldProps) => (
  <RAC.NumberField className={className} {...props}>
    {label && <FieldLabel>{label}</FieldLabel>}

    <RAC.Group
      className={composeTailwindRenderProps(
        groupClassName,
        focusVisibleRingStyles(),
        tw`flex min-w-0 rounded-md border border-neutral text-md leading-5 text-on-neutral`,
      )}
    >
      <RAC.Button className={tw`flex size-8 items-center justify-center border-r border-neutral`} slot='decrement'>
        <FiMinus />
      </RAC.Button>

      <RAC.Input className={tw`min-w-0 flex-1 px-3 outline-hidden`} />

      <RAC.Button className={tw`flex size-8 items-center justify-center border-l border-neutral`} slot='increment'>
        <FiPlus />
      </RAC.Button>
    </RAC.Group>
  </RAC.NumberField>
);
