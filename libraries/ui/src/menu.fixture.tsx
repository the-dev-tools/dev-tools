import { MenuTrigger } from 'react-aria-components';
import { FiMoreHorizontal, FiPlus } from 'react-icons/fi';

import { Avatar, AvatarProps } from './avatar';
import { Button } from './button';
import { ListBoxHeader } from './list-box';
import { Menu, MenuItem, MenuItemProps } from './menu';
import { Separator } from './separator';
import { tw } from './tailwind-literal';

const basic = (
  <MenuTrigger>
    <Button className={tw`p-1`}>
      <FiMoreHorizontal className={tw`size-4 stroke-[1.2px] text-slate-500`} />
    </Button>

    <Menu aria-label='Menu'>
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

const avatars = (
  <MenuTrigger>
    <Button className={tw`p-1`}>
      <FiMoreHorizontal className={tw`size-4 stroke-[1.2px] text-slate-500`} />
    </Button>

    <Menu aria-label='Menu with Avatars'>
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

export default {
  basic,
  'with-avatars': avatars,
};
