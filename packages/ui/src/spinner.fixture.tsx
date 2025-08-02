import { useFixtureVariants } from '../cosmos/utils';
import { Spinner as Spinner_, SpinnerProps, spinnerStyles } from './spinner';

const Spinner = (props: SpinnerProps) => {
  const [variants] = useFixtureVariants(spinnerStyles);
  return <Spinner_ {...props} {...variants} />;
};

export default <Spinner />;
