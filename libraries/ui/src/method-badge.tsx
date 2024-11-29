import { Match, pipe } from 'effect';
import { tv } from 'tailwind-variants';

import { Badge, BadgeProps } from './badge';
import { tw } from './tailwind-literal';

type Method = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE' | 'HEAD' | 'OPTION' | 'CONNECT' | (string & {});

type MatchedMethod = [string, BadgeProps['color']];

const styles = tv({
  variants: {
    size: {
      default: tw`w-10`,
      lg: tw`w-12`,
    },
  },
  defaultVariants: {
    size: 'default',
  },
});

export interface MethodBadgeProps extends Omit<BadgeProps, 'color' | 'children'> {
  method: Method;
}

export const MethodBadge = ({ method, className, ...props }: MethodBadgeProps) => {
  const [value, color] = pipe(
    Match.value(method),
    Match.when('GET', (_): MatchedMethod => [_ as string, 'green']),
    Match.when('POST', (_): MatchedMethod => [_ as string, 'amber']),
    Match.when('PUT', (_): MatchedMethod => [_ as string, 'sky']),
    Match.when('PATCH', (): MatchedMethod => ['PAT', 'purple']),
    Match.when('DELETE', (): MatchedMethod => ['DEL', 'rose']),
    Match.when('HEAD', (_): MatchedMethod => [_ as string, 'blue']),
    Match.when('OPTION', (): MatchedMethod => ['OPT', 'fuchsia']),
    Match.when('CONNECT', (): MatchedMethod => ['CON', 'slate']),
    Match.orElse((_): MatchedMethod => [_, 'slate']),
  );

  return (
    <Badge color={color} className={styles({ className, size: props.size })} {...props}>
      {value}
    </Badge>
  );
};
