import { createContext, Dispatch, ReactNode, SetStateAction } from 'react';

export interface FlowContext {
  flowId: Uint8Array;
  isReadOnly?: boolean;
  setSidebar?: Dispatch<SetStateAction<ReactNode>>;
}

export const FlowContext = createContext({} as FlowContext);
