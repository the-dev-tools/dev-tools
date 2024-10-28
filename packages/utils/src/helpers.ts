// https://github.com/microsoft/TypeScript/issues/13948#issuecomment-1333159066
// eslint-disable-next-line @typescript-eslint/no-unsafe-return, @typescript-eslint/no-explicit-any
export const keyValue = <K extends PropertyKey, V>(k: K, v: V): { [P in K]: Record<P, V> }[K] => ({ [k]: v }) as any;
