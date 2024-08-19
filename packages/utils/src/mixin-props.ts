import { pipe, String, Types } from 'effect';

export type MixinProps<Mixin extends string, Props> = {
  [Key in keyof Props as `${Mixin}${Capitalize<Key & string>}`]: Props[Key];
};

type SplitProps<Props, Mixins extends string[]> = {
  [Mixin in Mixins[number]]: {
    [MixinKey in keyof Props as MixinKey extends `${Mixin}${infer Key}` ? Uncapitalize<Key> : never]: Props[MixinKey];
  };
} & {
  rest: Types.Simplify<
    Omit<
      Props,
      {
        [Mixin in Mixins[number]]: keyof {
          [MixinKey in keyof Props as MixinKey extends `${Mixin}${string}` ? MixinKey : never]: never;
        };
      }[Mixins[number]]
    >
  >;
};

export const splitProps = <Props, Mixins extends string[]>(props: Props, ...mixins: Mixins) => {
  const result: Record<string, unknown> = {};
  const rest: Record<string, unknown> = {};
  for (const mixinKey of mixins) result[mixinKey] = {};

  for (const key in props) {
    let split = false;
    for (const mixinKey of mixins) {
      if (!String.startsWith(mixinKey)(key)) continue;
      split = true;
      const splitKey = pipe(key, String.substring(mixinKey.length), String.uncapitalize);
      const splitProps = result[mixinKey] as Record<string, unknown>;
      splitProps[splitKey] = props[key];
    }
    if (!split) rest[key] = props[key];
  }

  result['rest'] = rest;
  return result as SplitProps<Props, Mixins>;
};
