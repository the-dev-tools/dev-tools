import type { Meta, StoryObj } from '@storybook/react-vite';

import { HttpMethod } from '@the-dev-tools/spec/buf/api/http/v1/http_pb';
import { MethodBadge } from './method-badge';

const meta = {
  component: MethodBadge,

  args: { method: HttpMethod.GET },
} satisfies Meta<typeof MethodBadge>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};
