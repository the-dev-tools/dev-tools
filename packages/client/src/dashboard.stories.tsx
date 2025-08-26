import type { Meta, StoryObj } from '@storybook/react-vite';

import { tw } from '@the-dev-tools/ui/tailwind-literal';
import { DashboardLayout } from './dashboard';

const meta = {
  component: DashboardLayout,

  parameters: {
    layout: 'fullwidth',
  },
} satisfies Meta<typeof DashboardLayout>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    navbar: (
      <>
        <span>Home</span>
        <div className={tw`flex-1`} />
      </>
    ),
  },
};
