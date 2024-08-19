import { Record } from 'effect';

export const makeGoVariable = <T extends string>(id: T) => `{{ .${id} }}` as const;

export const makeGoVariables = <T extends string[]>(...ids: T) =>
  Record.fromIterableWith(ids, (id) => [id, makeGoVariable(id)] as const) as {
    [Key in T[number]]: ReturnType<typeof makeGoVariable<Key>>;
  };
