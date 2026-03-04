import { HandleKind } from '@the-dev-tools/spec/buf/api/flow/v1/flow_pb';
import { Ulid } from 'id128';
import { useMemo } from 'react';

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
  | { type: 'position'; nodes: { from: { x: number; y: number }; id: string; to: { x: number; y: number } }[] }
  | { type: 'node-delete'; flowId: Uint8Array; pasteOffset?: PasteOffset; yaml: string }
  | { type: 'edge-delete'; edges: EdgeSnapshot[] }
  | { type: 'edge-insert'; edgeIds: Uint8Array[]; edges: EdgeSnapshot[] }
  | { type: 'paste'; flowId: Uint8Array; nodeIds: Uint8Array[]; pasteOffset: PasteOffset; yaml: string };

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
  private executors: UndoExecutors | null = null;
  private executing = false;
  private undoEntries: UndoEntry[] = [];
  private redoEntries: UndoEntry[] = [];

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
  private async executeInverse(entry: UndoEntry): Promise<UndoEntry | null> {
    const exec = this.executors!;
    exec.deselectAll();

    switch (entry.type) {
      case 'position': {
        exec.updateNodePositions(entry.nodes.map((_) => ({ id: _.id, position: _.from })));
        return {
          type: 'position',
          nodes: entry.nodes.map((_) => ({ from: _.to, id: _.id, to: _.from })),
        };
      }

      case 'node-delete': {
        // Undo delete = paste the YAML back
        const pasteOffset = entry.pasteOffset ?? { x: 0, y: 0 };
        const newNodeIds = await exec.pasteNodes(entry.yaml, entry.flowId, pasteOffset);
        return { type: 'paste', flowId: entry.flowId, nodeIds: newNodeIds, pasteOffset, yaml: entry.yaml };
      }

      case 'paste': {
        // Undo paste = delete pasted nodes
        exec.deleteNodes(entry.nodeIds);
        return { type: 'node-delete', flowId: entry.flowId, pasteOffset: entry.pasteOffset, yaml: entry.yaml };
      }

      case 'edge-delete': {
        // Undo edge delete = re-insert edges, inverse is edge-insert (to delete them again on redo)
        const newEdgeIds = entry.edges.map((_) => exec.insertEdge(_));
        return { type: 'edge-insert', edgeIds: newEdgeIds, edges: entry.edges };
      }

      case 'edge-insert': {
        // Undo edge insert = delete the inserted edges, inverse is edge-delete (to re-insert on redo)
        exec.deleteEdges(entry.edgeIds);
        return { type: 'edge-delete', edges: entry.edges };
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
