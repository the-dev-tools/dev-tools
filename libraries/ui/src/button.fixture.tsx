import { useFixtureVariants } from '../cosmos/utils';
import { Button as Button_, ButtonProps, buttonStyles, buttonVariantKeys } from './button';

const Button = (props: ButtonProps) => {
  const [variants] = useFixtureVariants(buttonStyles, buttonVariantKeys);
  return <Button_ {...props} {...variants} />;
};

export default <Button isDisabled={false}>Button</Button>;
