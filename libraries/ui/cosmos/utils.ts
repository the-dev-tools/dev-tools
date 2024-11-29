import { Option, Record, Tuple } from 'effect';
import { useFixtureSelect } from 'react-cosmos/client';
import { ClassValue, TVDefaultVariants, TVVariants } from 'tailwind-variants';

type TVSlots = Record<string, ClassValue> | undefined;

export const useFixtureVariants = <
  V extends TVVariants<S>,
  S extends TVSlots,
  EV extends TVVariants<ES>,
  ES extends TVSlots,
  Key extends keyof V = keyof V,
>(
  styles: {
    variants: V;
    defaultVariants: TVDefaultVariants<V, S, EV, ES>;
  },
  keys?: Key[],
) => {
  const states = Record.filterMap(styles.variants, (values, variant) => {
    if (keys !== undefined && !keys.includes(variant as Key)) return Option.none();

    const options = Record.keys(values);
    const defaultValue = styles.defaultVariants[variant as keyof typeof styles.variants] as string;

    // eslint-disable-next-line react-hooks/rules-of-hooks
    return Option.some(useFixtureSelect(variant, { options, defaultValue }));
  });

  const values = Record.map(states, Tuple.getFirst) as { [Variant in Key]: keyof V[Variant] };
  const setters = Record.map(states, Tuple.getSecond) as { [Variant in Key]: (value: keyof V[Variant]) => void };

  return [values, setters, {} as Key] as const;
};
