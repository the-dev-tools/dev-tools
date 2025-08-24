import type { Meta, StoryObj } from '@storybook/react-vite';

import { MethodBadge } from './method-badge';

const meta = {
  component: MethodBadge,

  args: { method: 'GET' },
} satisfies Meta<typeof MethodBadge>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};
