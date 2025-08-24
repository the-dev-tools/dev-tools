import type { Meta, StoryObj } from '@storybook/react-vite';
import { fn } from 'storybook/test';

import { ButtonAsLink } from './button';

const meta = {
  component: ButtonAsLink,

  args: {
    children: 'Button',
    onPress: fn(),
  },
} satisfies Meta<typeof ButtonAsLink>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};
