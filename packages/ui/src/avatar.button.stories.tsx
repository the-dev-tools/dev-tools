import type { Meta, StoryObj } from '@storybook/react-vite';
import { fn } from 'storybook/test';

import { AvatarButton } from './avatar';

const meta = {
  component: AvatarButton,

  args: {
    children: 'Avatar',
    onPress: fn(),
  },
} satisfies Meta<typeof AvatarButton>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};
