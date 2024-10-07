import { UseControllerProps } from 'react-hook-form';

export const controllerPropKeys = [
  'control',
  'defaultValue',
  'disabled',
  'name',
  'rules',
  'shouldUnregister',
] as const satisfies (keyof UseControllerProps)[];

export type ControllerPropKeys = (typeof controllerPropKeys)[number];
