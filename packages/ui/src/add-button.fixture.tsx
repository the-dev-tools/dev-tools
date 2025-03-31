import { twJoin } from 'tailwind-merge';

import { useFixtureVariants } from '../cosmos/utils';
import { AddButton as AddButton_, AddButtonProps, addButtonStyles, addButtonVariantKeys } from './add-button';
import { tw } from './tailwind-literal';

const AddButton = (props: AddButtonProps) => {
  const [variants] = useFixtureVariants(addButtonStyles, addButtonVariantKeys);
  return (
    <div className={twJoin(tw`p-2`, variants.variant === 'light' && tw`bg-slate-950`)}>
      <AddButton_ {...props} {...variants} />{' '}
    </div>
  );
};

export default <AddButton />;
