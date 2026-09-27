// Builds the API Recorder extension with plain Vite (no extension framework).
//
// Output (same layout the release workflow and Chrome Web Store listing expect):
//   dist/chrome-mv3-prod/      unpacked extension, loadable via chrome://extensions
//   dist/chrome-mv3-prod.zip   Chrome Web Store upload
//
// The manifest is generated from package.json (`displayName`, `version`,
// `author` and the `manifest` block), so version bumps via Nx release keep
// working and permissions stay declared in one place.

import TailwindVite from '@tailwindcss/vite';
import ReactVite from '@vitejs/plugin-react';
import * as fs from 'node:fs/promises';
import * as path from 'node:path';
import * as zlib from 'node:zlib';
import { build, type InlineConfig } from 'vite';

import pkg from './package.json' with { type: 'json' };

const projectRoot = import.meta.dirname;
const srcDir = path.join(projectRoot, 'src');
const distDir = path.join(projectRoot, 'dist');
const outDir = path.join(distDir, 'chrome-mv3-prod');
const zipPath = path.join(distDir, 'chrome-mv3-prod.zip');

const shared = {
  configFile: false,
  define: { 'process.env.NODE_ENV': JSON.stringify('production') },
  logLevel: 'warn',
  mode: 'production',
  resolve: { alias: [{ find: /^~\/?/, replacement: `${srcDir}/` }] },
  root: srcDir,
} satisfies InlineConfig;

// Popup and auth callback pages
await build({
  ...shared,
  base: './',
  build: {
    // Loaded from the extension package, not the network, so large chunks are fine
    chunkSizeWarningLimit: 2048,
    emptyOutDir: true,
    modulePreload: { polyfill: false },
    outDir,
    rollupOptions: {
      input: {
        popup: path.join(srcDir, 'popup.html'),
        'tabs/auth-callback': path.join(srcDir, 'tabs', 'auth-callback.html'),
      },
    },
    target: 'chrome120',
  },
  plugins: [ReactVite(), TailwindVite()],
  publicDir: path.join(projectRoot, 'public'),
});

// Background service worker, bundled into a single classic script so the
// manifest does not need `"type": "module"`
await build({
  ...shared,
  build: {
    copyPublicDir: false,
    emptyOutDir: false,
    lib: {
      entry: path.join(srcDir, 'background.ts'),
      fileName: () => 'static/background/index.js',
      formats: ['iife'],
      name: 'background',
    },
    outDir,
    target: 'chrome120',
  },
});

const icons = Object.fromEntries([16, 32, 48, 64, 128].map((size) => [size.toString(), `icon${size.toString()}.png`]));

// Key order matches the manifest of the currently published version
/* eslint-disable perfectionist/sort-objects */
const manifest = {
  icons,
  manifest_version: 3,
  action: { default_icon: icons, default_popup: 'popup.html' },
  version: pkg.version,
  author: pkg.author,
  name: pkg.displayName,
  background: { service_worker: 'static/background/index.js' },
  ...pkg.manifest,
};
/* eslint-enable perfectionist/sort-objects */

await fs.writeFile(path.join(outDir, 'manifest.json'), JSON.stringify(manifest, null, 2) + '\n');

// Store upload zip. Minimal deterministic zip writer (deflate, fixed timestamps)
// so we don't need an extra dependency or a system `zip` binary.

const listFiles = async (dir: string): Promise<string[]> => {
  const entries = await fs.readdir(dir, { withFileTypes: true });
  const nested = await Promise.all(
    entries.map((entry) => {
      const full = path.join(dir, entry.name);
      return entry.isDirectory() ? listFiles(full) : Promise.resolve([full]);
    }),
  );
  return nested.flat();
};

const writeZip = async (sourceDir: string, target: string) => {
  const files = (await listFiles(sourceDir)).sort();
  const locals: Buffer[] = [];
  const centrals: Buffer[] = [];
  let offset = 0;

  // DOS date for 1980-01-01 00:00, keeps the zip byte-for-byte reproducible
  const dosTime = 0;
  const dosDate = (1 << 5) | 1;

  for (const file of files) {
    const name = Buffer.from(path.relative(sourceDir, file).split(path.sep).join('/'));
    const data = await fs.readFile(file);
    const compressed = zlib.deflateRawSync(data, { level: 9 });
    const crc = zlib.crc32(data);

    const local = Buffer.alloc(30);
    local.writeUInt32LE(0x04034b50, 0);
    local.writeUInt16LE(20, 4);
    local.writeUInt16LE(0x0800, 6); // UTF-8 names
    local.writeUInt16LE(8, 8); // deflate
    local.writeUInt16LE(dosTime, 10);
    local.writeUInt16LE(dosDate, 12);
    local.writeUInt32LE(crc, 14);
    local.writeUInt32LE(compressed.length, 18);
    local.writeUInt32LE(data.length, 22);
    local.writeUInt16LE(name.length, 26);
    local.writeUInt16LE(0, 28);

    const central = Buffer.alloc(46);
    central.writeUInt32LE(0x02014b50, 0);
    central.writeUInt16LE(20, 4);
    central.writeUInt16LE(20, 6);
    central.writeUInt16LE(0x0800, 8);
    central.writeUInt16LE(8, 10);
    central.writeUInt16LE(dosTime, 12);
    central.writeUInt16LE(dosDate, 14);
    central.writeUInt32LE(crc, 16);
    central.writeUInt32LE(compressed.length, 20);
    central.writeUInt32LE(data.length, 24);
    central.writeUInt16LE(name.length, 28);
    central.writeUInt32LE(offset, 42);

    locals.push(local, name, compressed);
    centrals.push(central, name);
    offset += local.length + name.length + compressed.length;
  }

  const centralDir = Buffer.concat(centrals);
  const end = Buffer.alloc(22);
  end.writeUInt32LE(0x06054b50, 0);
  end.writeUInt16LE(files.length, 8);
  end.writeUInt16LE(files.length, 10);
  end.writeUInt32LE(centralDir.length, 12);
  end.writeUInt32LE(offset, 16);

  await fs.writeFile(target, Buffer.concat([...locals, centralDir, end]));
  return files.length;
};

const count = await writeZip(outDir, zipPath);
console.log(
  `Built ${pkg.displayName} ${pkg.version}: ${path.relative(projectRoot, outDir)} (${count.toString()} files)`,
);
console.log(`Store upload: ${path.relative(projectRoot, zipPath)}`);
