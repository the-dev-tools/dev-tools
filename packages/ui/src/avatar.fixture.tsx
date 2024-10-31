import { useFixtureVariants } from '../cosmos/utils';
import { Avatar as Avatar_, AvatarProps, avatarStyles } from './avatar';

const Avatar = (props: AvatarProps) => {
  const [variants] = useFixtureVariants(avatarStyles);
  return <Avatar_ {...props} {...variants} />;
};

export default <Avatar shorten>Avatar</Avatar>;
