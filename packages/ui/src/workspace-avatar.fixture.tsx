import { Record, Struct } from 'effect';
import { useFixtureSelect } from 'react-cosmos/client';

import { WorkspaceAvatar, workspaceAvatarStyles } from './workspace-avatar';

const Fixture = () => {
  const [variant] = useFixtureSelect('variant', {
    options: Record.keys(workspaceAvatarStyles.variants.variant),
    ...Struct.pick(workspaceAvatarStyles.defaultVariants, 'variant'),
  });

  return <WorkspaceAvatar variant={variant}>Workspace Avatar</WorkspaceAvatar>;
};

export default Fixture;
