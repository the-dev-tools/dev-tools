import { Logger, LogLevel, ManagedRuntime } from 'effect';

export const Runtime = ManagedRuntime.make(Logger.minimumLogLevel(LogLevel.Debug));
