import type { Meta, StoryObj } from '@storybook/react-vite';

import * as RAC from 'react-aria-components';
import { TreeItem } from './tree';

const meta = {
  parameters: { layout: 'padded' },
} satisfies Meta;

export default meta;

type Story = StoryObj<typeof meta>;

export const Basic: Story = {
  render: function Render() {
    return (
      <RAC.Tree aria-label='Tree'>
        <TreeItem
          item={
            <>
              <TreeItem
                item={
                  <>
                    <TreeItem>Item A.A.A</TreeItem>
                    <TreeItem>Item A.A.B</TreeItem>
                    <TreeItem>Item A.A.C</TreeItem>
                  </>
                }
              >
                Item A.A
              </TreeItem>
              <TreeItem>Item A.B</TreeItem>
              <TreeItem>Item A.C</TreeItem>
            </>
          }
        >
          Item A
        </TreeItem>
        <TreeItem>Item B</TreeItem>
        <TreeItem>Item C</TreeItem>
      </RAC.Tree>
    );
  },
};

export const Infinite: Story = {
  render: function Render() {
    return (
      <RAC.Tree aria-label='Tree'>
        <InfiniteTreeItem>Item A</InfiniteTreeItem>
        <InfiniteTreeItem>Item B</InfiniteTreeItem>
        <InfiniteTreeItem>Item C</InfiniteTreeItem>
      </RAC.Tree>
    );
  },
};

interface InfiniteTreeItemProps {
  children: string;
}

const InfiniteTreeItem = ({ children }: InfiniteTreeItemProps) => {
  return (
    <TreeItem
      item={({ id }) => <InfiniteTreeItem>{`${children}.${id}`}</InfiniteTreeItem>}
      items={[{ id: 'A' }, { id: 'B' }, { id: 'C' }]}
    >
      {children}
    </TreeItem>
  );
};
