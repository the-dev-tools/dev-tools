import type { Meta, StoryObj } from '@storybook/react-vite';
import { fn } from 'storybook/test';

import { MenuTrigger } from 'react-aria-components';
import { FiMoreHorizontal, FiPlus } from 'react-icons/fi';
import { Avatar, AvatarProps } from './avatar';
import { Button } from './button';
import { ListBoxHeader } from './list-box';
import { Menu, MenuItem, MenuItemProps, MenuItemRouteLink } from './menu';
import { Separator } from './separator';
import { tw } from './tailwind-literal';

const meta = {
  component: Menu,
  subcomponents: { MenuItem, MenuItemRouteLink, MenuTrigger, Separator },
  tags: ['autodocs'],

  args: {
    'aria-label': 'Menu',
    onAction: fn(),
  },
} satisfies Meta<typeof Menu>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Basic: Story = {
  render: function Render(props) {
    return (
      <MenuTrigger>
        <Button className={tw`p-1`}>
          <FiMoreHorizontal className={tw`size-4 stroke-[1.2px] text-slate-500`} />
        </Button>

        <Menu {...props}>
          <MenuItem>Open in tab</MenuItem>
          <MenuItem>Add example</MenuItem>
          <Separator />
          <MenuItem>Share</MenuItem>
          <MenuItem>Copy Link</MenuItem>
          <MenuItem>Rename</MenuItem>
          <MenuItem>Duplicate</MenuItem>
          <Separator />
          <MenuItem>View Documentation</MenuItem>
          <MenuItem variant='danger'>Delete</MenuItem>
        </Menu>
      </MenuTrigger>
    );
  },
};

const MenuItemAvatar = ({
  children,
  color,
  ...props
}: Omit<MenuItemProps, 'children' | 'className'> & { children: string; color?: AvatarProps['variant'] }) => (
  <MenuItem {...props} className={tw`font-semibold`} textValue={children}>
    <Avatar shape='square' size='base' variant={color}>
      {children}
    </Avatar>
    {children}
  </MenuItem>
);

export const WithAvatars: Story = {
  render: function Render(props) {
    return (
      <MenuTrigger>
        <Button className={tw`p-1`}>
          <FiMoreHorizontal className={tw`size-4 stroke-[1.2px] text-slate-500`} />
        </Button>

        <Menu {...props}>
          <ListBoxHeader>Your Workspace</ListBoxHeader>
          <MenuItemAvatar color='violet'>Workspace 1.1</MenuItemAvatar>
          <MenuItemAvatar color='lime'>KreativeDesk</MenuItemAvatar>
          <MenuItemAvatar color='amber'>Keystone Workspace</MenuItemAvatar>
          <MenuItemAvatar color='blue'>QuestHub</MenuItemAvatar>
          <MenuItemAvatar color='pink'>TrendSpace</MenuItemAvatar>
          <Separator />
          <MenuItem variant='accent'>
            <FiPlus className={tw`stroke-[1.2px]`} /> Create Workspace
          </MenuItem>
        </Menu>
      </MenuTrigger>
    );
  },
};
