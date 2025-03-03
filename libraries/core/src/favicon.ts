import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { pipe } from 'effect';
import type { HtmlTagDescriptor, Plugin, ResolvedConfig } from 'vite';

// TODO: generate favicons programmatically with https://github.com/itgalaxy/favicons

export const FaviconPlugin = (): Plugin => {
  let config: ResolvedConfig;
  let tags: HtmlTagDescriptor[];
  return {
    name: 'favicon-plugin',
    configResolved: (_) => void (config = _),
    transformIndexHtml: () => tags,
    buildStart() {
      const favicon = (fileName: string) => {
        const url = import.meta.resolve(`@the-dev-tools/core/assets/favicon/${fileName}`);
        if (config.command === 'serve') return url;
        this.emitFile({ type: 'asset', fileName, source: pipe(url, fileURLToPath, readFileSync) });
        return fileName;
      };

      tags = [
        { tag: 'link', attrs: { rel: 'icon', type: 'image/png', href: favicon('favicon-96x96.png'), sizes: '96x96' } },
        { tag: 'link', attrs: { rel: 'icon', type: 'image/svg+xml', href: favicon('favicon.svg') } },
        { tag: 'link', attrs: { rel: 'shortcut icon', href: favicon('favicon.ico') } },
        { tag: 'link', attrs: { rel: 'apple-touch-icon', sizes: '180x180', href: favicon('apple-touch-icon.png') } },
        { tag: 'meta', attrs: { name: 'apple-mobile-web-app-title', content: 'DevTools' } },
        { tag: 'link', attrs: { rel: 'manifest', href: favicon('site.webmanifest') } },
      ];
    },
  };
};
