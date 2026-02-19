import { NodeContext } from '@effect/platform-node';
import { Effect, pipe } from 'effect';
import { authEffect } from './auth-effect.ts';

export const auth = pipe(authEffect, Effect.provide(NodeContext.layer), Effect.runSync);
