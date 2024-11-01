import { FiPlus } from 'react-icons/fi';

import { Avatar, AvatarProps } from './avatar';
import { ListBox, ListBoxHeader, ListBoxItem, ListBoxItemProps } from './list-box';
import { Separator } from './separator';
import { tw } from './tailwind-literal';

// eslint-disable-next-line @typescript-eslint/no-empty-function
const noop = () => {};

const basic = (
  <ListBox aria-label='List Box'>
    <ListBoxItem onAction={noop}>Open in tab</ListBoxItem>
    <ListBoxItem onAction={noop}>Add example</ListBoxItem>
    <Separator />
    <ListBoxItem onAction={noop}>Share</ListBoxItem>
    <ListBoxItem onAction={noop}>Copy Link</ListBoxItem>
    <ListBoxItem onAction={noop}>Rename</ListBoxItem>
    <ListBoxItem onAction={noop}>Duplicate</ListBoxItem>
    <Separator />
    <ListBoxItem onAction={noop}>View Documentation</ListBoxItem>
    <ListBoxItem onAction={noop} variant='danger'>
      Delete
    </ListBoxItem>
  </ListBox>
);

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

const avatars = (
  <ListBox aria-label='List Box with Avatars' selectionMode='single' disallowEmptySelection>
    <ListBoxHeader>Your Workspace</ListBoxHeader>
    <ListBoxItemAvatar color='violet'>Workspace 1.1</ListBoxItemAvatar>
    <ListBoxItemAvatar color='lime'>KreativeDesk</ListBoxItemAvatar>
    <ListBoxItemAvatar color='amber'>Keystone Workspace</ListBoxItemAvatar>
    <ListBoxItemAvatar color='blue'>QuestHub</ListBoxItemAvatar>
    <ListBoxItemAvatar color='pink'>TrendSpace</ListBoxItemAvatar>
    <Separator />
    <ListBoxItem variant='accent' onAction={noop}>
      <FiPlus className={tw`stroke-[1.2px]`} /> Create Workspace
    </ListBoxItem>
  </ListBox>
);

export default {
  basic,
  'with-avatars': avatars,
};
