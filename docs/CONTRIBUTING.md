# Contributing Guidelines

Pull requests, bug reports, and all other forms of contribution are welcomed and highly encouraged!

Reading and following these guidelines will help us make the contribution process easy and effective for everyone involved. It also communicates that you agree to respect the time of the developers managing and developing these open source projects. In return, we will reciprocate that respect by addressing your issue, assessing changes, and helping you finalize your pull requests.

<details>
  <summary>Table of Contents</summary>
  <ol>
    <li><a href="#code-of-conduct">Code of Conduct</a></li>
    <li><a href="#getting-started">Getting Started</a></li>
    <li><a href="#issues">Issues</a></li>
    <li><a href="#pull-requests">Pull Requests</a></li>
    <li><a href="#environment-setup">Environment Setup</a></li>
    <li>
      <a href="#tooling-references">Tooling References</a>
      <ul>
        <li><a href="#general">General</a></li>
        <li><a href="#server">Server</a></li>
        <li><a href="#client">Client</a></li>
      </ul>
    </li>
  </ol>
</details>

## Code of Conduct

We take our open source community seriously and hold ourselves and other contributors to high standards of communication. By participating and contributing to this project, you agree to uphold the [Contributor Covenant Code of Conduct](CODE-OF-CONDUCT.md).

## Getting Started

Contributions are made to this repo via Issues and Pull Requests (PRs). A few general guidelines that cover both:

- To report security vulnerabilities, please contact us directly at [help@dev.tools](mailto:help@dev.tools)
- Search for existing Issues and PRs before creating your own
- We work hard to makes sure issues are handled in a timely manner but, depending on the impact, it could take a while to investigate the root cause. A friendly ping in the comment thread to the submitter or a contributor can help draw attention if your issue is blocking

## Issues

Issues should be used to report a problem, request a new feature, or to discuss potential changes before a PR is created. When you create a new Issue, a template will be loaded that will guide you through collecting and providing the information we need to investigate.

If you find an Issue that addresses the problem you're having, please add your own reproduction information to the existing issue rather than creating a new one. Adding a [reaction](https://github.blog/2016-03-10-add-reactions-to-pull-requests-issues-and-comments/) can also help be indicating to our maintainers that a particular problem is affecting more than just the reporter.

## Pull Requests

PRs are always welcome and can be a quick way to get your fix or improvement slated for the next release. In general, PRs should:

- Only fix/add the functionality in question
- Address a single concern in the least number of changed lines as possible
- Include a [Version Plan](https://nx.dev/recipes/nx-release/file-based-versioning-version-plans#create-version-plans) describing the changes and semantic versioning information. You can run `nx release plan` to generate it with an interactive guide
- Be accompanied by a complete Pull Request template (loaded automatically when a PR is created)

For changes that address core functionality or would require breaking changes (e.g. a major release), it's best to open an Issue to discuss your proposal first. This is not required but can save time creating and reviewing changes.

In general, we follow the ["fork-and-pull" Git workflow](https://github.com/susam/gitpr)

1. Fork the repository to your own GitHub account
2. Clone the project to your machine
3. Create a branch locally with a succinct but descriptive name
4. Commit changes to the branch
5. Ensure all formatting and testing checks pass by running `task lint` and `task test`
6. Push changes to your fork
7. Open a PR in our repository and follow the PR template so that we can efficiently review the changes

## Environment Setup

The development environment for this project is set up using Nix Flakes for full reproducibility. You may chooose to set your environment manually, but we won't be able to help you if issues arise. For best results set up the following software:

1. [Nix](https://nixos.org/) with Flakes enabled. We recommend using [Determinate Nix Installer](https://github.com/DeterminateSystems/nix-installer) for good defaults
2. [Direnv](https://direnv.net/), see [installation instructions](https://direnv.net/docs/installation.html). Run `direnv allow` in project root to activate the environment
3. [Visual Studio Code](https://code.visualstudio.com/) with [recommended extensions](https://code.visualstudio.com/docs/editor/extension-marketplace#_recommended-extensions)

Make sure to update project dependencies by running `pnpm install` and `scripts go-install-tools` before making any changes.

## Tooling References

This is a list of tools that we frequently use throughout the project, along with accompanying references. It is helpful for quickly looking up certain information during development.

### General

- Nix Flakes - [Wiki](https://wiki.nixos.org/wiki/Flakes) - [Install](https://github.com/DeterminateSystems/nix-installer#readme)
- direnv - [Docs](https://direnv.net/) - [Install](https://direnv.net/docs/installation.html)
- pnpm - [Docs](https://pnpm.io/motivation)
- Nx - [Docs](https://nx.dev/getting-started/intro) - [Plugins](https://nx.dev/plugin-registry) - [API](https://nx.dev/nx-api)

### RPC

- Connect RPC - [Docs](https://connectrpc.com/docs/introduction)
- Protobuf - [Docs](https://protobuf.dev/)

### Server

#### Database

- Libsql - Unix like - [Website](https://turso.tech/libsql) - [Github](https://github.com/tursodatabase/go-libsql)
- Sqlite - Windows - [Website](https://www.sqlite.org/) - [Go Reference](https://pkg.go.dev/github.com/mattn/go-sqlite3)

#### RPC

- Connect for Go [Docs](https://connectrpc.com/docs/go/getting-started)

#### General

- Ulid - [Go Reference](https://pkg.go.dev/github.com/oklog/ulid)
- GVal - [Go Reference](https://pkg.go.dev/github.com/PaesslerAG/gval)
- Compressed - [Go Reference](https://pkg.go.dev/github.com/klauspost/compress)

### Client

#### General

- Effect - [Docs](https://effect.website/docs/) - [API](https://effect-ts.github.io/effect/docs/effect)
  - Schema - [Docs](https://effect.website/docs/schema/introduction/) - [API](https://effect-ts.github.io/effect/effect/Schema.ts.html)
  - Platform - [Docs](https://effect.website/docs/platform/introduction/) - [API](https://effect-ts.github.io/effect/docs/platform)
- Faker - [API](https://fakerjs.dev/api/)
- React Email - [Docs](https://react.email/docs/introduction) - [Components](https://react.email/components) - [Templates](https://react.email/templates)
- Electron Vite - [Docs](https://electron-vite.org/guide/)

#### RPC

- Connect for Web - [Docs](https://connectrpc.com/docs/web/getting-started)
- Connect for TanStack Query - [Docs](https://github.com/connectrpc/connect-query-es)
- Protobuf ES - [Docs](https://github.com/bufbuild/protobuf-es/blob/main/MANUAL.md)

#### React

- TanStack Router - [Docs](https://tanstack.com/router/latest/docs/framework/react/overview)
- TanStack Query - [Docs](https://tanstack.com/query/latest/docs/framework/react/overview)
- TanStack Table - [Docs](https://tanstack.com/table/latest/docs/introduction)
- normy - [Core](https://github.com/klis87/normy#readme) - [TanStack Query](https://github.com/klis87/normy/tree/master/packages/normy-react-query#readme)
- React Hook Form - [API](https://react-hook-form.com/docs) - [TS](https://react-hook-form.com/ts)
- React Flow - [Docs](https://reactflow.dev/learn) - [API](https://reactflow.dev/api-reference) - [Components](https://reactflow.dev/components) - [Examples](https://reactflow.dev/examples)

#### UI

- Tailwind CSS - [Docs](https://tailwindcss.com/docs/installation)
- Tailwind Variants - [Docs](https://www.tailwind-variants.org/docs/introduction)
- React Aria - [Docs](https://react-spectrum.adobe.com/react-aria/components.html)
  - Tailwind Starter - [GitHub](https://github.com/adobe/react-spectrum/tree/main/starters/tailwind) - [Storybook](https://react-spectrum.adobe.com/react-aria-tailwind-starter/)
- React Icons - [Docs](https://react-icons.github.io/react-icons)

#### Design

<!-- TODO: probably move to the private repository -->

- Figma - [Team](https://www.figma.com/files/team/1400037238435055305/all-projects) - [File](https://www.figma.com/design/psOxuc1CnTJTklIvga49To/DevTools)
- Token Studio - [Docs](https://docs.tokens.studio/)
