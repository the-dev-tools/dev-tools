import { Ulid } from 'id128';
import { useMemo } from 'react';
import { HandleKind } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface EdgeSnapshot {
  flowId: Uint8Array;
  sourceHandle: HandleKind;
  sourceId: Uint8Array;
  targetId: Uint8Array;
}

interface PasteOffset {
  x: number;
  y: number;
}

export type UndoEntry =
  | { edgeIds: Uint8Array[]; edges: EdgeSnapshot[]; type: 'edge-insert' }
  | { edges: EdgeSnapshot[]; type: 'edge-delete' }
  | { flowId: Uint8Array; nodeIds: Uint8Array[]; pasteOffset: PasteOffset; type: 'paste'; yaml: string }
  | { flowId: Uint8Array; pasteOffset?: PasteOffset; type: 'node-delete'; yaml: string }
  | { nodes: { from: { x: number; y: number }; id: string; to: { x: number; y: number } }[]; type: 'position' };

export interface UndoExecutors {
  deleteEdges(edgeIds: Uint8Array[]): void;
  deleteNodes(nodeIds: Uint8Array[]): void;
  deselectAll(): void;
  insertEdge(edge: EdgeSnapshot): Uint8Array;
  pasteNodes(yaml: string, flowId: Uint8Array, offset: PasteOffset): Promise<Uint8Array[]>;
  updateNodePositions(nodes: { id: string; position: { x: number; y: number } }[]): void;
}

// ---------------------------------------------------------------------------
// UndoStack
// ---------------------------------------------------------------------------

const MAX_STACK_SIZE = 50;

export class UndoStack {
  private executing = false;
  private executors: null | UndoExecutors = null;
  private redoEntries: UndoEntry[] = [];
  private undoEntries: UndoEntry[] = [];

  setExecutors(executors: UndoExecutors) {
    this.executors = executors;
  }

  push(entry: UndoEntry) {
    if (this.executing) return;
    this.undoEntries.push(entry);
    if (this.undoEntries.length > MAX_STACK_SIZE) this.undoEntries.shift();
    this.redoEntries = [];
  }

  async undo() {
    if (this.executing || !this.executors || this.undoEntries.length === 0) return;
    this.executing = true;
    try {
      const entry = this.undoEntries.pop()!;
      const redoEntry = await this.executeInverse(entry);
      if (redoEntry) this.redoEntries.push(redoEntry);
    } finally {
      this.executing = false;
    }
  }

  async redo() {
    if (this.executing || !this.executors || this.redoEntries.length === 0) return;
    this.executing = true;
    try {
      const entry = this.redoEntries.pop()!;
      const undoEntry = await this.executeInverse(entry);
      if (undoEntry) this.undoEntries.push(undoEntry);
    } finally {
      this.executing = false;
    }
  }

  clear() {
    this.undoEntries = [];
    this.redoEntries = [];
  }

  get canUndo() {
    return this.undoEntries.length > 0;
  }

  get canRedo() {
    return this.redoEntries.length > 0;
  }

  // Execute the inverse of an entry, returning a new entry for the opposite stack
  private async executeInverse(entry: UndoEntry): Promise<null | UndoEntry> {
    const exec = this.executors!;
    exec.deselectAll();

    switch (entry.type) {
      case 'edge-delete': {
        // Undo edge delete = re-insert edges, inverse is edge-insert (to delete them again on redo)
        const newEdgeIds = entry.edges.map((_) => exec.insertEdge(_));
        return { edgeIds: newEdgeIds, edges: entry.edges, type: 'edge-insert' };
      }

      case 'edge-insert': {
        // Undo edge insert = delete the inserted edges, inverse is edge-delete (to re-insert on redo)
        exec.deleteEdges(entry.edgeIds);
        return { edges: entry.edges, type: 'edge-delete' };
      }

      case 'node-delete': {
        // Undo delete = paste the YAML back
        const pasteOffset = entry.pasteOffset ?? { x: 0, y: 0 };
        const newNodeIds = await exec.pasteNodes(entry.yaml, entry.flowId, pasteOffset);
        return { flowId: entry.flowId, nodeIds: newNodeIds, pasteOffset, type: 'paste', yaml: entry.yaml };
      }

      case 'paste': {
        // Undo paste = delete pasted nodes
        exec.deleteNodes(entry.nodeIds);
        return { flowId: entry.flowId, pasteOffset: entry.pasteOffset, type: 'node-delete', yaml: entry.yaml };
      }

      case 'position': {
        exec.updateNodePositions(entry.nodes.map((_) => ({ id: _.id, position: _.from })));
        return {
          nodes: entry.nodes.map((_) => ({ from: _.to, id: _.id, to: _.from })),
          type: 'position',
        };
      }
    }
  }
}

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

const stacks = new Map<string, UndoStack>();

export const useUndoStack = (flowId: Uint8Array): UndoStack => {
  const key = Ulid.construct(flowId).toCanonical();
  return useMemo(() => {
    let stack = stacks.get(key);
    if (!stack) {
      stack = new UndoStack();
      stacks.set(key, stack);
    }
    return stack;
  }, [key]);
};
