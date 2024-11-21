import { NodeContext } from '@effect/platform-node';
import { ManagedRuntime } from 'effect';

export const layer = NodeContext.layer;

export const Runtime = ManagedRuntime.make(layer);
