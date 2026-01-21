/**
 * Effect Schema definitions for execution (control) tools
 * These tools manage workflow execution: run, stop, validate
 */

import { Schema } from 'effect';

import { FlowId } from './common.ts';

// =============================================================================
// Execution Control Schemas
// =============================================================================

/**
 * Run a workflow from the start
 */
export const RunWorkflow = Schema.Struct({
  flowId: FlowId.pipe(
    Schema.annotations({
      description: 'The ULID of the workflow to run',
    }),
  ),
}).pipe(
  Schema.annotations({
    identifier: 'runWorkflow',
    title: 'Run Workflow',
    description: 'Execute the workflow from the start node. Returns execution status.',
  }),
);

/**
 * Stop a running workflow
 */
export const StopWorkflow = Schema.Struct({
  flowId: FlowId.pipe(
    Schema.annotations({
      description: 'The ULID of the workflow to stop',
    }),
  ),
}).pipe(
  Schema.annotations({
    identifier: 'stopWorkflow',
    title: 'Stop Workflow',
    description: 'Stop a running workflow execution.',
  }),
);

/**
 * Validate workflow before running
 */
export const ValidateWorkflow = Schema.Struct({
  flowId: FlowId.pipe(
    Schema.annotations({
      description: 'The ULID of the workflow to validate',
    }),
  ),
}).pipe(
  Schema.annotations({
    identifier: 'validateWorkflow',
    title: 'Validate Workflow',
    description:
      'Validate the workflow for errors, missing connections, or configuration issues. Use this before running to catch problems.',
  }),
);

// =============================================================================
// Exports
// =============================================================================

export const ExecutionSchemas = {
  RunWorkflow,
  StopWorkflow,
  ValidateWorkflow,
} as const;

export type RunWorkflow = typeof RunWorkflow.Type;
export type StopWorkflow = typeof StopWorkflow.Type;
export type ValidateWorkflow = typeof ValidateWorkflow.Type;
