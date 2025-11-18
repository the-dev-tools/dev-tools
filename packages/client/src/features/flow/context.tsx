import { createContext } from 'react';

export interface FlowContext {
  flowId: Uint8Array;
  isReadOnly?: boolean;
}

export const FlowContext = createContext({} as FlowContext);
