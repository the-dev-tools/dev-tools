<p align="center">
  <a href="https://dev.tools/">
    <img width=200px height=200px src="./apps/api-recorder-extension/assets/icon.png">
  </a>
</p>

<h3 align="center">The Dev Tools</h3>

<div align="center">

[![Chrome Web Store Version](https://img.shields.io/chrome-web-store/v/bcnbbkdpnoeaaedhhnlefgpijlpbmije?logo=googlechrome&logoColor=white&label=API%20Recorder%20Extension)](https://chromewebstore.google.com/detail/api-recorder/bcnbbkdpnoeaaedhhnlefgpijlpbmije)

</div>

## Getting Started

The following software is needed for the development environment:

1. [Nix](https://nixos.org/) with Flakes enabled, see [installation instructions](https://github.com/DeterminateSystems/nix-installer)
2. [Direnv](https://direnv.net/), see [installation instructions](https://direnv.net/docs/installation.html)
3. [Visual Studio Code](https://code.visualstudio.com/) with [recommended extensions](https://code.visualstudio.com/docs/editor/extension-marketplace#_recommended-extensions)

> [!IMPORTANT]
> When opening the project for the first time, run `direnv allow` and `pnpm install`

## Resources

### Development Environment

- Nix Flakes - [Wiki](https://wiki.nixos.org/wiki/Flakes) - [Install](https://github.com/DeterminateSystems/nix-installer#readme)
- direnv - [Docs](https://direnv.net/) - [Install](https://direnv.net/docs/installation.html)
- pnpm - [Docs](https://pnpm.io/motivation)
- Nx - [Docs](https://nx.dev/getting-started/intro) - [Plugins](https://nx.dev/plugin-registry) - [API](https://nx.dev/nx-api)
- dotenvx - [Docs](https://dotenvx.com/docs/)

### General

- Effect - [Docs](https://effect.website/docs/) - [API](https://effect-ts.github.io/effect/docs/effect)
  - Schema - [Docs](https://effect.website/docs/schema/introduction/) - [API](https://effect-ts.github.io/effect/effect/Schema.ts.html)
  - Platform - [Docs](https://effect.website/docs/platform/introduction/) - [API](https://effect-ts.github.io/effect/docs/platform)
- Faker - [API](https://fakerjs.dev/api/)

### State Management

- TanStack Router - [Docs](https://tanstack.com/router/latest/docs/framework/react/overview)
- TanStack Query - [Docs](https://tanstack.com/query/latest/docs/framework/react/overview)
- TanStack Table - [Docs](https://tanstack.com/table/latest/docs/introduction)
- normy - [Core](https://github.com/klis87/normy#readme) - [TanStack Query](https://github.com/klis87/normy/tree/master/packages/normy-react-query#readme)
- React Hook Form - [API](https://react-hook-form.com/docs) - [TS](https://react-hook-form.com/ts)
- React Flow - [Docs](https://reactflow.dev/learn) - [API](https://reactflow.dev/api-reference) - [Components](https://reactflow.dev/components) - [Examples](https://reactflow.dev/examples)

### RPC / Protobufs

- Connect for Web - [Docs](https://connectrpc.com/docs/web/getting-started)
- Connect for TanStack Query - [Docs](https://github.com/connectrpc/connect-query-es)
- Protobuf ES - [Docs](https://github.com/bufbuild/protobuf-es/blob/main/MANUAL.md)

### UI

- Tailwind CSS - [Docs](https://tailwindcss.com/docs/installation)
- Tailwind Variants - [Docs](https://www.tailwind-variants.org/docs/introduction)
- React Aria - [Docs](https://react-spectrum.adobe.com/react-aria/components.html)
  - Tailwind Starter - [GitHub](https://github.com/adobe/react-spectrum/tree/main/starters/tailwind) - [Storybook](https://react-spectrum.adobe.com/react-aria-tailwind-starter/)
- React Icons - [Docs](https://react-icons.github.io/react-icons)

### Design

- Figma - [Team](https://www.figma.com/files/team/1400037238435055305/all-projects) - [File](https://www.figma.com/design/psOxuc1CnTJTklIvga49To/DevTools)
- Token Studio - [Docs](https://docs.tokens.studio/)

### Other

- React Email - [Docs](https://react.email/docs/introduction) - [Components](https://react.email/components) - [Templates](https://react.email/templates)
- Electron Vite - [Docs](https://electron-vite.org/guide/)
