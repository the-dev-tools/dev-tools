import { useFixtureInput } from 'react-cosmos/client';
import { useFixtureVariants } from '../cosmos/utils';
import { Button as Button_, ButtonProps, buttonStyles, buttonVariantKeys } from './button';

const Button = (props: ButtonProps) => {
  const [variants] = useFixtureVariants(buttonStyles, buttonVariantKeys);
  const [isPending] = useFixtureInput('isPending', false);
  return <Button_ isPending={isPending} {...props} {...variants} />;
};

export default <Button isDisabled={false}>Button</Button>;
