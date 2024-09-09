import 'ag-grid-community/styles/ag-grid.css';
import './ag-grid.css';

import { AgGridReact, AgGridReactProps, CustomCellEditorProps, CustomCellRendererProps } from 'ag-grid-react';
import { useEffect } from 'react';
import { twMerge } from 'tailwind-merge';

import { MixinProps, splitProps } from '@the-dev-tools/utils/mixin-props';

import { Checkbox, CheckboxBox, CheckboxIndicator } from './checkbox';
import { tw } from './tailwind-literal';

export interface AgGridBasicProps<TData>
  extends AgGridReactProps<TData>,
    MixinProps<'wrapper', Omit<React.ComponentProps<'div'>, 'children'>> {}

export const AgGridBasic = <TData,>({
  wrapperClassName,
  defaultColDef,
  components,
  ...props
}: AgGridBasicProps<TData>) => {
  const forwardedProps = splitProps(props, 'wrapper');
  return (
    <div {...forwardedProps.wrapper} className={twMerge(tw`ag-theme-devtools`, wrapperClassName)}>
      <AgGridReact
        {...forwardedProps.rest}
        defaultColDef={{
          sortable: false,
          suppressMovable: true,
          flex: 1,
          ...defaultColDef,
        }}
        components={{
          agCheckboxCellRenderer: CheckboxCellRenderer,
          agCheckboxCellEditor: CheckboxCellEditor,
          ...components,
        }}
      />
    </div>
  );
};

const CheckboxCellRenderer = ({ value, setValue, eGridCell }: CustomCellRendererProps<unknown, boolean>) => {
  useEffect(() => {
    const listener = (event: KeyboardEvent) => {
      if (event.key !== ' ') return;
      setValue?.(!value);
      event.stopPropagation();
    };
    eGridCell.addEventListener('keydown', listener);
    return () => void eGridCell.removeEventListener('keydown', listener);
  }, [eGridCell, setValue, value]);

  return (
    <button
      className='flex size-full items-center'
      onClick={(event) => {
        event.stopPropagation();
        setValue?.(!value);
      }}
    >
      <CheckboxBox>
        <CheckboxIndicator isSelected={value ?? false} />
      </CheckboxBox>
    </button>
  );
};

const CheckboxCellEditor = ({ value, onValueChange }: CustomCellEditorProps<unknown, boolean>) => (
  <Checkbox
    isSelected={value ?? false}
    onChange={onValueChange}
    className='size-full'
    style={{
      paddingLeft: 'calc(var(--ag-cell-horizontal-padding) - 1px)',
      paddingRight: 'calc(var(--ag-cell-horizontal-padding) - 1px)',
    }}
    // eslint-disable-next-line jsx-a11y/no-autofocus
    autoFocus
  />
);
