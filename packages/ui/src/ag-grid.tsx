import 'ag-grid-community/styles/ag-grid.css';
import './ag-grid.css';

import { AgGridReact, AgGridReactProps } from 'ag-grid-react';
import { twMerge } from 'tailwind-merge';

import { MixinProps, splitProps } from '@the-dev-tools/utils/mixin-props';

import { tw } from './tailwind-literal';

export interface AgGridBasicProps<TData>
  extends AgGridReactProps<TData>,
    MixinProps<'wrapper', Omit<React.ComponentProps<'div'>, 'children'>> {}

export const AgGridBasic = <TData,>({ wrapperClassName, defaultColDef, ...props }: AgGridBasicProps<TData>) => {
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
      />
    </div>
  );
};
