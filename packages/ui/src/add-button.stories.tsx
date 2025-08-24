import type { Meta, StoryObj } from '@storybook/react-vite';
import { fn } from 'storybook/test';

import { AddButton } from './add-button';

const meta = {
  component: AddButton,

  args: {
    onPress: fn(),
  },
} satisfies Meta<typeof AddButton>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};
