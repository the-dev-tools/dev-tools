import { Match, pipe } from 'effect';
import { tv } from 'tailwind-variants';
import { HttpMethod } from '@the-dev-tools/spec/api/http/v1/http_pb';
import { Badge, BadgeProps } from './badge';
import { tw } from './tailwind-literal';

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

export interface MethodBadgeProps extends Omit<BadgeProps, 'children' | 'color'> {
  method: HttpMethod;
}

export const MethodBadge = ({ className, method, ...props }: MethodBadgeProps) => {
  const [value, color] = pipe(
    Match.value(method),
    Match.when(HttpMethod.GET, (_): MatchedMethod => ['GET', 'green']),
    Match.when(HttpMethod.POST, (_): MatchedMethod => ['POST', 'amber']),
    Match.when(HttpMethod.PUT, (_): MatchedMethod => ['PUT', 'sky']),
    Match.when(HttpMethod.PATCH, (): MatchedMethod => ['PAT', 'purple']),
    Match.when(HttpMethod.DELETE, (): MatchedMethod => ['DEL', 'rose']),
    Match.when(HttpMethod.HEAD, (_): MatchedMethod => ['HEAD', 'blue']),
    Match.when(HttpMethod.OPTION, (): MatchedMethod => ['OPT', 'fuchsia']),
    Match.when(HttpMethod.CONNECT, (): MatchedMethod => ['CON', 'slate']),
    Match.orElse((_): MatchedMethod => ['N/A', 'slate']),
  );

  return (
    <Badge {...props} className={styles({ className, size: props.size })} color={color}>
      {value}
    </Badge>
  );
};
