import type { Meta, StoryObj } from '@storybook/react-vite';

import { fromJson } from '@bufbuild/protobuf';
import { ValueSchema } from '@bufbuild/protobuf/wkt';
import { Tree } from 'react-aria-components';
import { JsonTreeItem } from './json-tree';

const meta = {
  parameters: { layout: 'padded' },
} satisfies Meta;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: function Render() {
    return (
      <Tree aria-label='JSON Tree'>
        <JsonTreeItem
          jsonValue={fromJson(ValueSchema, {
            array: [1, 2, 3],
            boolean: true,
            item: 'value',
            number: 123,
            object: { a: 'a', b: 'b', c: 'c' },
          })}
        />
      </Tree>
    );
  },
};
