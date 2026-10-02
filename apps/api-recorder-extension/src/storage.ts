import { Effect, Option, pipe, Schema } from 'effect';

// Thin wrapper around `chrome.storage`, replacing `@plasmohq/storage`.
//
// Values are stored as JSON strings, exactly like `@plasmohq/storage` did, so
// data written by earlier published versions (recordings, login state) keeps
// working after an update.

export class Storage {
  readonly area: chrome.storage.AreaName;

  constructor({ area }: { area: chrome.storage.AreaName }) {
    this.area = area;
  }

  get storage() {
    return chrome.storage[this.area];
  }

  async get<T>(key: string): Promise<T | undefined> {
    const result = await this.storage.get(key);
    return parse(result[key]) as T | undefined;
  }

  async set(key: string, value: unknown) {
    await this.storage.set({ [key]: JSON.stringify(value) });
  }

  /** Subscribes to changes of the given keys. Returns an unsubscribe function. */
  watch(callbacks: Record<string, (change: { newValue?: unknown; oldValue?: unknown }) => void>) {
    const listener = (changes: Record<string, chrome.storage.StorageChange>, areaName: string) => {
      if (areaName !== this.area) return;
      for (const [key, change] of Object.entries(changes)) {
        const callback = callbacks[key];
        if (!callback) continue;
        const newValue = parse(change.newValue);
        const oldValue = parse(change.oldValue);
        callback({
          ...(newValue === undefined ? {} : { newValue }),
          ...(oldValue === undefined ? {} : { oldValue }),
        });
      }
    };
    chrome.storage.onChanged.addListener(listener);
    return () => void chrome.storage.onChanged.removeListener(listener);
  }
}

const parse = (value: unknown): unknown => {
  if (typeof value !== 'string') return value;
  try {
    return JSON.parse(value);
  } catch {
    return value;
  }
};

export const Local = new Storage({ area: 'local' });

export const Change = <S extends Schema.Schema.All>(schema: S) => {
  const value = pipe(schema, Schema.optionalWith({ as: 'Option' }));
  return Schema.Struct({ newValue: value, oldValue: value });
};

export const get = <T>(storage: Storage, key: string, schema: Schema.Schema<T>) =>
  Effect.gen(function* () {
    const value = yield* Effect.tryPromise(() => storage.get<typeof schema.Encoded>(key));
    if (value === undefined) return Option.none();
    return yield* Schema.decode(schema)(value).pipe(Effect.map(Option.some));
  });

export const set =
  <A, I>(storage: Storage, key: string, schema: Schema.Schema<A, I>) =>
  (value: A) =>
    pipe(
      Schema.encode(schema)(value),
      Effect.flatMap((_) => Effect.tryPromise(() => storage.set(key, _))),
    );
