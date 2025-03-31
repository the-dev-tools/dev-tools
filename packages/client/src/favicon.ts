import type { HtmlTagDescriptor, Plugin, ResolvedConfig } from 'vite';

import { pipe } from 'effect';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

// TODO: generate favicons programmatically with https://github.com/itgalaxy/favicons

export const FaviconPlugin = (): Plugin => {
  let config: ResolvedConfig;
  let tags: HtmlTagDescriptor[];
  return {
    buildStart() {
      const favicon = (fileName: string) => {
        const url = import.meta.resolve(`@the-dev-tools/client/assets/favicon/${fileName}`);
        if (config.command === 'serve') return url;
        this.emitFile({ fileName, source: pipe(url, fileURLToPath, readFileSync), type: 'asset' });
        return fileName;
      };

      tags = [
        { attrs: { href: favicon('favicon-96x96.png'), rel: 'icon', sizes: '96x96', type: 'image/png' }, tag: 'link' },
        { attrs: { href: favicon('favicon.svg'), rel: 'icon', type: 'image/svg+xml' }, tag: 'link' },
        { attrs: { href: favicon('favicon.ico'), rel: 'shortcut icon' }, tag: 'link' },
        { attrs: { href: favicon('apple-touch-icon.png'), rel: 'apple-touch-icon', sizes: '180x180' }, tag: 'link' },
        { attrs: { content: 'DevTools', name: 'apple-mobile-web-app-title' }, tag: 'meta' },
        { attrs: { href: favicon('site.webmanifest'), rel: 'manifest' }, tag: 'link' },
      ];
    },
    configResolved: (_) => void (config = _),
    name: 'favicon-plugin',
    transformIndexHtml: () => tags,
  };
};
