import { Record, Struct } from 'effect';
import { useFixtureInput, useFixtureSelect } from 'react-cosmos/client';

import { Button, buttonStyles } from './button';

const Fixture = () => {
  const [isDisabled] = useFixtureInput('isDisabled', false);
  const [variant] = useFixtureSelect('variant', {
    options: Record.keys(buttonStyles.variants.variant),
    ...Struct.pick(buttonStyles.defaultVariants, 'variant'),
  });

  return (
    <Button isDisabled={isDisabled} variant={variant}>
      Button
    </Button>
  );
};

export default Fixture;
