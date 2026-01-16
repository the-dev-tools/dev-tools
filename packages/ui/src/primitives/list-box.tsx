import { HKT } from 'effect';
import * as RAC from 'react-aria-components';
import { createLinkGeneric } from '../utils/link';

export interface ListBoxProps<T = object> extends RAC.ListBoxProps<T> {}

export const ListBox = <T extends object>(props: ListBoxProps<T>) => <RAC.ListBox {...props} />;

export interface ListBoxItemProps<T = object> extends RAC.ListBoxItemProps<T> {}

export const ListBoxItem = <T extends object>(props: ListBoxItemProps<T>) => <RAC.ListBoxItem {...props} />;

interface ListBoxItemTypeLambda extends HKT.TypeLambda {
  readonly type: typeof ListBoxItem<this['Target'] extends object ? this['Target'] : never>;
}

export const ListBoxItemRouteLink = createLinkGeneric<ListBoxItemTypeLambda, object>(ListBoxItem);
