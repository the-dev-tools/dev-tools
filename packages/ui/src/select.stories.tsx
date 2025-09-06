import type { Meta, StoryObj } from '@storybook/react-vite';
import { fn } from 'storybook/test';

import { Select, SelectItem } from './select';
import { Separator } from './separator';

const meta = {
  component: Select,
  subcomponents: { Select, SelectItem, Separator },

  args: { 'aria-label': 'Select' },
} satisfies Meta<typeof Select>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: { onSelectionChange: fn() },
  render: function Render(props) {
    return (
      <Select {...props}>
        <SelectItem>Item A</SelectItem>
        <SelectItem>Item B</SelectItem>
        <Separator />
        <SelectItem>Item C</SelectItem>
      </Select>
    );
  },
};
