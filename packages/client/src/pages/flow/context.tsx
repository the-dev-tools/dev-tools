import { createContext, Dispatch, ReactNode, SetStateAction } from 'react';
import { UndoStack } from './undo';

export interface FlowContext {
  agentPanelOpen?: boolean;
  flowId: Uint8Array;
  isReadOnly?: boolean;
  setAgentPanelOpen?: Dispatch<SetStateAction<boolean>>;
  setSidebar?: Dispatch<SetStateAction<ReactNode>>;
  undoStack?: UndoStack;
}

export const FlowContext = createContext({} as FlowContext);
