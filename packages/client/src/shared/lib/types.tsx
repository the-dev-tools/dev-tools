export type Filter<Value, Filter> = { [Key in keyof Value as Value[Key] extends Filter ? Key : never]: Value[Key] };

export type PartialUndefined<T> = { [K in keyof T]?: T[K] | undefined };
