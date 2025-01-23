// https://github.com/microsoft/TypeScript/issues/13948#issuecomment-1333159066
// eslint-disable-next-line @typescript-eslint/no-unsafe-return, @typescript-eslint/no-explicit-any
export const keyValue = <K extends PropertyKey, V>(k: K, v: V): { [P in K]: Record<P, V> }[K] => ({ [k]: v }) as any;

export const formatSize = (bytes: number) => {
  const scale = bytes == 0 ? 0 : Math.floor(Math.log(bytes) / Math.log(1024));
  const size = (bytes / Math.pow(1024, scale)).toFixed(2);
  const name = ['B', 'KiB', 'MiB', 'GiB', 'TiB'][scale];
  return `${size} ${name}`;
};
