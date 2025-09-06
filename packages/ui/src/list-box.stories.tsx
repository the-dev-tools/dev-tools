import type { Meta, StoryObj } from '@storybook/react-vite';
import { fn } from 'storybook/test';

import { FiPlus } from 'react-icons/fi';
import { Avatar, AvatarProps } from './avatar';
import { ListBox, ListBoxHeader, ListBoxItem, ListBoxItemProps } from './list-box';
import { Separator } from './separator';
import { tw } from './tailwind-literal';

const meta = {
  component: ListBox,
  subcomponents: { ListBoxHeader, ListBoxItem, Separator },
  tags: ['autodocs'],

  args: { 'aria-label': 'List Box' },
} satisfies Meta<typeof ListBox>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Basic: Story = {
  args: { onAction: fn() },
  render: function Render(props) {
    return (
      <ListBox {...props}>
        <ListBoxItem>Open in tab</ListBoxItem>
        <ListBoxItem>Add example</ListBoxItem>
        <Separator />
        <ListBoxItem>Share</ListBoxItem>
        <ListBoxItem>Copy Link</ListBoxItem>
        <ListBoxItem>Rename</ListBoxItem>
        <ListBoxItem>Duplicate</ListBoxItem>
        <Separator />
        <ListBoxItem>View Documentation</ListBoxItem>
        <ListBoxItem variant='danger'>Delete</ListBoxItem>
      </ListBox>
    );
  },
};

const ListBoxItemAvatar = ({
  children,
  color,
  ...props
}: Omit<ListBoxItemProps, 'children' | 'className'> & { children: string; color?: AvatarProps['variant'] }) => (
  <ListBoxItem {...props} className={tw`font-semibold`} textValue={children}>
    <Avatar shape='square' size='base' variant={color}>
      {children}
    </Avatar>
    {children}
  </ListBoxItem>
);

export const WithAvatars: Story = {
  args: {
    disallowEmptySelection: true,
    onSelectionChange: fn(),
    selectionMode: 'single',
  },
  render: function Render(props) {
    return (
      <ListBox {...props}>
        <ListBoxHeader>Your Workspace</ListBoxHeader>
        <ListBoxItemAvatar color='violet'>Workspace 1.1</ListBoxItemAvatar>
        <ListBoxItemAvatar color='lime'>KreativeDesk</ListBoxItemAvatar>
        <ListBoxItemAvatar color='amber'>Keystone Workspace</ListBoxItemAvatar>
        <ListBoxItemAvatar color='blue'>QuestHub</ListBoxItemAvatar>
        <ListBoxItemAvatar color='pink'>TrendSpace</ListBoxItemAvatar>
        <Separator />
        <ListBoxItem textValue='Create Workspace' variant='accent'>
          <FiPlus className={tw`stroke-[1.2px]`} /> Create Workspace
        </ListBoxItem>
      </ListBox>
    );
  },
};
