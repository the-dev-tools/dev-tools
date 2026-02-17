import type { EdgeInfo, NodeInfo } from './types';

export type LayoutOrientation = 'horizontal' | 'vertical';

export interface LayoutConfig {
  orientation: LayoutOrientation;
  spacingPrimary: number;
  spacingSecondary: number;
  startX: number;
  startY: number;
}

export interface Position {
  x: number;
  y: number;
}

export interface LayoutResult {
  levels: Map<string, number>;
  maxLevel: number;
  positions: Map<string, Position>;
}

export const defaultHorizontalConfig = (): LayoutConfig => ({
  orientation: 'horizontal',
  spacingPrimary: 300,
  spacingSecondary: 150,
  startX: 0,
  startY: 0,
});

export const defaultVerticalConfig = (): LayoutConfig => ({
  orientation: 'vertical',
  spacingPrimary: 300,
  spacingSecondary: 400,
  startX: 0,
  startY: 0,
});

const buildOutgoingAdjacency = (edges: EdgeInfo[]): Map<string, string[]> => {
  const adj = new Map<string, string[]>();
  for (const e of edges) {
    const existing = adj.get(e.sourceId) ?? [];
    existing.push(e.targetId);
    adj.set(e.sourceId, existing);
  }
  return adj;
};

const findStartNode = (nodes: NodeInfo[]): NodeInfo | undefined => nodes.find((n) => n.kind === 'ManualStart');

/**
 * Layout computes node positions using BFS-based level assignment.
 * Each node's level is max(parent_levels) + 1, ensuring proper dependency ordering.
 * Cycles are handled by only visiting each node once.
 */
export const layout = (
  nodes: NodeInfo[],
  edges: EdgeInfo[],
  startNodeId: string,
  config: LayoutConfig,
): LayoutResult => {
  if (nodes.length === 0) {
    return {
      levels: new Map(),
      maxLevel: 0,
      positions: new Map(),
    };
  }

  const outgoingEdges = buildOutgoingAdjacency(edges);

  const nodeLevels = new Map<string, number>();
  const levelNodes = new Map<number, string[]>();
  const visited = new Set<string>();

  // Start BFS from start node
  const queue: string[] = [startNodeId];
  nodeLevels.set(startNodeId, 0);
  levelNodes.set(0, [startNodeId]);
  visited.add(startNodeId);

  while (queue.length > 0) {
    const currentNodeId = queue.shift()!;
    const currentLevel = nodeLevels.get(currentNodeId) ?? 0;

    // Process all children
    const children = outgoingEdges.get(currentNodeId) ?? [];
    for (const childId of children) {
      // Skip if already visited (handles cycles)
      if (visited.has(childId)) continue;

      // Child level is parent level + 1
      const childLevel = currentLevel + 1;

      // Mark as visited and assign level
      visited.add(childId);
      nodeLevels.set(childId, childLevel);
      const currentLevelNodes = levelNodes.get(childLevel) ?? [];
      currentLevelNodes.push(childId);
      levelNodes.set(childLevel, currentLevelNodes);
      queue.push(childId);
    }
  }

  // Find max level
  let maxLevel = 0;
  for (const level of levelNodes.keys()) {
    if (level > maxLevel) maxLevel = level;
  }

  // Calculate positions based on orientation
  const positions = new Map<string, Position>();

  for (let level = 0; level <= maxLevel; level++) {
    const nodesAtLevel = levelNodes.get(level) ?? [];
    if (nodesAtLevel.length === 0) continue;

    // Calculate primary axis position (depth direction)
    let primaryPos = config.orientation === 'horizontal' ? config.startX : config.startY;
    primaryPos += level * config.spacingPrimary;

    // Calculate secondary axis positions (centered around start)
    const totalSecondary = (nodesAtLevel.length - 1) * config.spacingSecondary;
    let startSecondary = config.orientation === 'horizontal' ? config.startY : config.startX;
    startSecondary -= totalSecondary / 2;

    for (let i = 0; i < nodesAtLevel.length; i++) {
      const nodeId = nodesAtLevel[i]!;
      const secondaryPos = startSecondary + i * config.spacingSecondary;

      const pos: Position =
        config.orientation === 'horizontal' ? { x: primaryPos, y: secondaryPos } : { x: secondaryPos, y: primaryPos };

      positions.set(nodeId, pos);
    }
  }

  return {
    levels: nodeLevels,
    maxLevel,
    positions,
  };
};

/**
 * layoutNodes is a convenience function that finds the start node and performs layout.
 * Returns null if no start node is found.
 */
export const layoutNodes = (
  nodes: NodeInfo[],
  edges: EdgeInfo[],
  config: LayoutConfig = defaultHorizontalConfig(),
): LayoutResult | null => {
  const startNode = findStartNode(nodes);
  if (!startNode) return null;

  return layout(nodes, edges, startNode.id, config);
};
