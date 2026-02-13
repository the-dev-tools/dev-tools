import type { Meta, StoryObj } from '@storybook/react-vite';
import { fn } from 'storybook/test';

import { Dialog, DialogTrigger } from 'react-aria-components';
import { Button } from './button';
import { Modal } from './modal';
import { tw } from './tailwind-literal';

const meta = {
  component: Modal,

  args: { onOpenChange: fn() },
} satisfies Meta<typeof Modal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: { size: 'sm' },
  render: function Render({ onOpenChange, ...props }) {
    return (
      <DialogTrigger onOpenChange={onOpenChange!}>
        <Button>Open Modal</Button>

        <Modal {...props}>
          <Dialog className={tw`flex h-full flex-col p-4 outline-hidden`}>
            <h1 className={tw`mb-1 leading-5 font-semibold tracking-tight text-on-neutral`}>Delete workspace?</h1>
            <span className={tw`text-sm leading-5 font-medium tracking-tight text-on-neutral-low`}>
              This action will remove the workspace permanently
            </span>

            <div className={tw`flex-1`} />

            <div className={tw`mt-5 flex justify-end gap-2`}>
              <Button slot='close' variant='secondary'>
                Cancel
              </Button>
              <Button className={tw`border-danger bg-danger-low`} slot='close' variant='primary'>
                Delete
              </Button>
            </div>
          </Dialog>
        </Modal>
      </DialogTrigger>
    );
  },
};
