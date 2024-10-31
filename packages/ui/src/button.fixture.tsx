import { Record } from 'effect';
import { useFixtureSelect } from 'react-cosmos/client';

import { Button as Button_, ButtonProps, buttonStyles } from './button';

const Button = (props: ButtonProps) => {
  const [variant] = useFixtureSelect('variant', {
    options: Record.keys(buttonStyles.variants.variant),
    defaultValue: props.variant ?? buttonStyles.defaultVariants.variant!,
  });

  return (
    <Button_ {...props} variant={variant}>
      Button
    </Button_>
  );
};

export default <Button isDisabled={false}>Button</Button>;
