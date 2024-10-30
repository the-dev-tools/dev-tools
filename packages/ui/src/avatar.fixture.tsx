import { Record, Struct } from 'effect';
import { useFixtureSelect } from 'react-cosmos/client';

import { Avatar, avatarStyles } from './avatar';

const Fixture = () => {
  const [variant] = useFixtureSelect('variant', {
    options: Record.keys(avatarStyles.variants.variant),
    ...Struct.pick(avatarStyles.defaultVariants, 'variant'),
  });

  return <Avatar variant={variant}>Avatar</Avatar>;
};

export default Fixture;
