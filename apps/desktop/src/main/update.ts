import { HttpClient, HttpClientResponse } from '@effect/platform';
import { CustomPublishOptions } from 'builder-util-runtime';
import { Array, Effect, Exit, pipe, Runtime, Schema } from 'effect';
import { AppUpdater, UpdateInfo, Provider as UpdateProvider } from 'electron-updater';
import { ProviderRuntimeOptions, resolveFiles } from 'electron-updater/out/providers/Provider';
import * as Yaml from 'yaml';

declare module 'builder-util-runtime' {
  interface CustomPublishOptions {
    project: {
      name: string;
      path: string;
    };
    repo: string;
    runtime: Runtime.Runtime<Effect.Effect.Context<ReturnType<typeof getUpdateInfo>>>;
  }
}

export const getUpdateInfo = Effect.fn(function* (options: CustomPublishOptions) {
  const client = pipe(yield* HttpClient.HttpClient, HttpClient.followRedirects(3));

  const { version } = yield* pipe(
    client.get(
      `https://raw.githubusercontent.com/${options.repo}/refs/heads/main/${options.project.path}/package.json`,
    ),
    Effect.flatMap(HttpClientResponse.schemaBodyJson(Schema.Struct({ version: Schema.String }))),
  );

  const { assets } = yield* pipe(
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
        }),
      ),
    ),
  );

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
  constructor(
    readonly options: CustomPublishOptions,
    readonly updater: AppUpdater,
    runtimeOptions: ProviderRuntimeOptions,
  ) {
    super(runtimeOptions);
  }

  async getLatestVersion() {
    const result = await pipe(getUpdateInfo(this.options), Runtime.runPromiseExit(this.options.runtime));

    return Exit.match(result, {
      onFailure: (): UpdateInfo => ({
        files: [],
        path: '',
        releaseDate: '',
        sha512: '',
        version: this.updater.currentVersion as string,
      }),
      onSuccess: (_) => _,
    });
  }

  resolveFiles(updateInfo: UpdateInfo) {
    return resolveFiles(
      updateInfo,
      new URL(
        `https://github.com/${this.options.repo}/releases/download/${this.options.project.name}@${updateInfo.version}/`,
      ),
    );
  }
}
