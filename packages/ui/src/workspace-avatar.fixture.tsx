import { Record } from 'effect';
import { useFixtureSelect } from 'react-cosmos/client';

import { WorkspaceAvatar as WorkspaceAvatar_, WorkspaceAvatarProps, workspaceAvatarStyles } from './workspace-avatar';

const Fixture = (props: WorkspaceAvatarProps) => {
  const [variant] = useFixtureSelect('variant', {
    options: Record.keys(workspaceAvatarStyles.variants.variant),
    defaultValue: props.variant ?? workspaceAvatarStyles.defaultVariants.variant!,
  });

  return <WorkspaceAvatar_ {...props} variant={variant} />;
};

export default <Fixture>Workspace Avatar</Fixture>;
