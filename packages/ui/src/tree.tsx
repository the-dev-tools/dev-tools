import { type CollectionProps } from '@react-aria/collections';
import { HKT } from 'effect';
import { ComponentProps, ReactNode, RefAttributes, useState } from 'react';
import * as RAC from 'react-aria-components';
import { FiMove } from 'react-icons/fi';
import { Button } from './button';
import { focusVisibleRingStyles } from './focus-ring';
import { ChevronSolidDownIcon } from './icons';
import { Spinner } from './spinner';
import { tw } from './tailwind-literal';
import { composeTailwindRenderProps } from './utils';
import { createLinkGeneric } from './utils/link';

export interface TreeItemProps<T extends object>
  extends Omit<RAC.TreeItemProps, 'children' | 'textValue'>, RefAttributes<HTMLDivElement> {
  childItems?: ReactNode;
  children: RAC.TreeItemContentProps['children'];
  isExpanded?: boolean;
  isLoading?: boolean;
  item?: CollectionProps<T>['children'];
  items?: T[];
  onContextMenu?: ComponentProps<'div'>['onContextMenu'];
  onExpand?: () => void;
  setIsExpanded?: (value: boolean) => void;
  textValue?: RAC.TreeItemProps['textValue'];
}

export const TreeItem = <T extends object>({
  childItems: childItemsProps,
  children,
  className,
  isExpanded: controlledIsExpanded,
  isLoading,
  item,
  items,
  onContextMenu,
  onExpand,
  ref,
  setIsExpanded: controlledSetIsExpanded,
  textValue,
  ...props
}: TreeItemProps<T>) => {
  const [defaultIsExpanded, defaultSetIsExpanded] = useState(false);
  const isExpanded = controlledIsExpanded ?? defaultIsExpanded;
  const setIsExpanded = controlledSetIsExpanded ?? defaultSetIsExpanded;

  let childItems = childItemsProps;
  if (item && !items) childItems = <RAC.Collection>{isExpanded ? item : null}</RAC.Collection>;
  if (item && items) childItems = <RAC.Collection items={isExpanded ? items : []}>{item}</RAC.Collection>;
  if (childItems)
    childItems = (
      <>
        {childItems}
        <RAC.TreeLoadMoreItem />
      </>
    );

  return (
    <RAC.TreeItem
      {...props}
      className={composeTailwindRenderProps(
        className,
        focusVisibleRingStyles(),
        tw`
          group/tree-item cursor-pointer rounded-md bg-transparent px-3 py-1.5 text-md leading-5 font-medium
          tracking-tight text-slate-800

          hover:bg-slate-100

          active:bg-slate-200

          pressed:bg-slate-200

          selected:bg-slate-200

          drop-target:bg-violet-200
        `,
      )}
      ref={(node) => {
        if (!node) return;

        if (typeof ref === 'object') ref = { current: node };
        if (typeof ref === 'function') ref(node);

        const handler = () => {
          const isExpanded = node.attributes.getNamedItem('data-expanded')?.value === 'true';
          if (isExpanded) onExpand?.();
          setIsExpanded(isExpanded);
        };
        handler();
        const observer = new MutationObserver(handler);
        observer.observe(node, { attributeFilter: ['data-expanded'] });
        return () => void observer.disconnect();
      }}
      textValue={textValue ?? (typeof children === 'string' ? children : undefined!)}
    >
      <RAC.TreeItemContent>
        {RAC.composeRenderProps(children, (children, { allowsDragging, hasChildItems, level }) => {
          let icon = <div className={tw`size-5 shrink-0`} />;
          if (isLoading) icon = <Spinner className={tw`size-5 p-1`} />;
          else if (hasChildItems)
            icon = (
              <RAC.Button className={tw`shrink-0 cursor-pointer`} slot='chevron'>
                <ChevronSolidDownIcon
                  className={tw`
                    size-5 rotate-0 p-1 text-slate-500 transition-transform

                    group-expanded/tree-item:rotate-90
                  `}
                />
              </RAC.Button>
            );

          return (
            <div
              className={tw`relative z-0 flex items-center gap-2`}
              onContextMenu={onContextMenu}
              style={{ paddingInlineStart: ((level - 1) * (20 / 16)).toString() + 'rem' }}
            >
              {icon}
              {children}
              {allowsDragging && (
                <Button className={tw`absolute right-0 -z-10 p-1 opacity-0 focus:z-10 focus:opacity-100`} slot='drag'>
                  <FiMove className={tw`size-3 text-slate-500`} />
                </Button>
              )}
            </div>
          );
        })}
      </RAC.TreeItemContent>

      {childItems}
    </RAC.TreeItem>
  );
};

export interface TreeItemTypeLambda extends HKT.TypeLambda {
  readonly type: typeof TreeItem<this['Target'] extends object ? this['Target'] : never>;
}

export const TreeItemRouteLink = createLinkGeneric<TreeItemTypeLambda, object>(TreeItem);
