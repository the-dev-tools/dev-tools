import { createContext, Dispatch, ReactNode, SetStateAction } from 'react';

export interface FlowContext {
  agentPanelOpen?: boolean;
  flowId: Uint8Array;
  isReadOnly?: boolean;
  setAgentPanelOpen?: Dispatch<SetStateAction<boolean>>;
  setSidebar?: Dispatch<SetStateAction<ReactNode>>;
}

export const FlowContext = createContext({} as FlowContext);
