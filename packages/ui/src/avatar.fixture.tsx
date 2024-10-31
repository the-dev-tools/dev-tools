import { Record } from 'effect';
import { useFixtureSelect } from 'react-cosmos/client';

import { Avatar as Avatar_, AvatarProps, avatarStyles } from './avatar';

const Avatar = (props: AvatarProps) => {
  const [variant] = useFixtureSelect('variant', {
    options: Record.keys(avatarStyles.variants.variant),
    defaultValue: props.variant ?? avatarStyles.defaultVariants.variant!,
  });

  return <Avatar_ {...props} variant={variant} />;
};

export default <Avatar shorten>Avatar</Avatar>;
