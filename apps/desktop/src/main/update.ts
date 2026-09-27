import { HttpClient, HttpClientResponse } from '@effect/platform';
import { CustomPublishOptions } from 'builder-util-runtime';
import { Array, Effect, Exit, pipe, Runtime, Schema } from 'effect';
import { AppUpdater, UpdateInfo, Provider as UpdateProvider } from 'electron-updater';
import { ProviderRuntimeOptions, resolveFiles } from 'electron-updater/out/providers/Provider';
import * as Yaml from 'yaml';

export interface UpdateOptions {
  project: {
    name: string;
    path: string;
  };
  repo: string;
  runtime: Runtime.Runtime<Effect.Effect.Context<ReturnType<typeof getUpdateInfo>>>;
}

declare module 'builder-util-runtime' {
  interface CustomPublishOptions {
    update?: UpdateOptions;
  }
}

// Semver pre-release: a "-" in the version before any "+build" metadata,
// e.g. 1.2.0-0 or 1.2.0-rc.1.
const isPrereleaseVersion = (version: string) => version.split('+')[0]?.includes('-') ?? false;

const getUpdateInfo = Effect.fn(function* (options: UpdateOptions, allowPrerelease: boolean) {
  const client = pipe(yield* HttpClient.HttpClient, HttpClient.followRedirects(3));

  const { version } = yield* pipe(
    client.get(
      `https://raw.githubusercontent.com/${options.repo}/refs/heads/main/${options.project.path}/package.json`,
    ),
    Effect.flatMap(HttpClientResponse.schemaBodyJson(Schema.Struct({ version: Schema.String }))),
  );

  const { assets, prerelease } = yield* pipe(
    client.get(`https://api.github.com/repos/${options.repo}/releases/tags/${options.project.name}@${version}`),
    Effect.flatMap(
      HttpClientResponse.schemaBodyJson(
        Schema.Struct({
          assets: Schema.Array(
            Schema.Struct({
              browser_download_url: Schema.String,
              name: Schema.String,
            }),
          ),
          prerelease: Schema.optional(Schema.Boolean),
        }),
      ),
    ),
  );

  // Never offer a pre-release to a stable install. electron-updater only
  // applies `allowPrerelease` in its built-in GitHub provider, not in this
  // custom one, so enforce it here. It defaults to true only when the running
  // app is itself a pre-release build. Failing here resolves to "no update"
  // in getLatestVersion below.
  if (!allowPrerelease && (prerelease === true || isPrereleaseVersion(version))) {
    return yield* Effect.fail(new Error(`Skipping pre-release ${options.project.name}@${version}`));
  }

  const updateInfoAsset = yield* Array.findFirst(
    assets,
    (_) => _.name === `latest-${process.platform}-${process.arch}.yml`,
  );

  return yield* pipe(
    client.get(updateInfoAsset.browser_download_url),
    Effect.flatMap((_) => _.text),
    Effect.flatMap((_) => Effect.try(() => Yaml.parse(_) as UpdateInfo)),
  );
});

export class CustomUpdateProvider extends UpdateProvider<UpdateInfo> {
  readonly updateOptions: UpdateOptions;

  constructor(
    readonly options: CustomPublishOptions,
    readonly updater: AppUpdater,
    runtimeOptions: ProviderRuntimeOptions,
  ) {
    super(runtimeOptions);

    if (!options.update) throw new Error('Update options must be provided');
    this.updateOptions = options.update;
  }

  async getLatestVersion() {
    const result = await pipe(
      getUpdateInfo(this.updateOptions, this.updater.allowPrerelease),
      Runtime.runPromiseExit(this.updateOptions.runtime),
    );

    return Exit.match(result, {
      onFailure: (): UpdateInfo => ({
        files: [],
        path: '',
        releaseDate: '',
        sha512: '',
        version: this.updater.currentVersion.raw,
      }),
      onSuccess: (_) => _,
    });
  }

  resolveFiles(updateInfo: UpdateInfo) {
    return resolveFiles(
      updateInfo,
      new URL(
        `https://github.com/${this.updateOptions.repo}/releases/download/${this.updateOptions.project.name}@${updateInfo.version}/`,
      ),
    );
  }
}
