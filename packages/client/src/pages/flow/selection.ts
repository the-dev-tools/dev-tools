import * as XF from '@xyflow/react';
import { useCallback } from 'react';

/**
 * Centralized selection API for flow nodes and edges.
 *
 * All selection writes go through ReactFlow's store actions
 * (`addSelectedNodes`, `unselectNodesAndEdges`, `triggerNodeChanges`)
 * to keep both `nodeLookup` and client collections in sync.
 */
export const useFlowSelection = () => {
  const storeApi = XF.useStoreApi();

  const selectedNodeIds = XF.useStore(
    (s) => s.nodes.filter((n) => n.selected).map((n) => n.id),
    (a, b) => a.length === b.length && a.every((id, i) => id === b[i]),
  );

  /** Select nodes exclusively — deselects all other nodes and edges first. */
  const selectNodes = useCallback(
    (ids: string[]) => {
      const { addSelectedNodes } = storeApi.getState();
      addSelectedNodes(ids);
    },
    [storeApi],
  );

  const deselectNodes = useCallback(
    (ids: string[]) => {
      const { triggerNodeChanges } = storeApi.getState();
      triggerNodeChanges(ids.map((id) => ({ id, selected: false, type: 'select' as const })));
    },
    [storeApi],
  );

  const deselectAll = useCallback(() => {
    const { unselectNodesAndEdges } = storeApi.getState();
    unselectNodesAndEdges();
  }, [storeApi]);

  return { deselectAll, deselectNodes, selectedNodeIds, selectNodes };
};
