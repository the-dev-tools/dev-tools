export const controllerPropKeys = ['name', 'control', 'defaultValue', 'rules', 'shouldUnregister', 'disabled'] as const;

export type ControllerPropKeys = (typeof controllerPropKeys)[number];
