<p align="center">
  <a href="https://dev.tools/">
    <img width=200px height=200px src="./assets/icon.png">
  </a>
</p>

<h3 align="center">API Recorder</h3>

<div align="center">

[![Chrome Web Store Version](https://img.shields.io/chrome-web-store/v/bcnbbkdpnoeaaedhhnlefgpijlpbmije?logo=googlechrome&logoColor=white)](https://chromewebstore.google.com/detail/api-recorder/bcnbbkdpnoeaaedhhnlefgpijlpbmije)

</div>

## Getting Started

The following software is needed for the development environment:

1. [Nix](https://nixos.org/) with Flakes enabled, see [installation instructions](https://github.com/DeterminateSystems/nix-installer)
2. [Direnv](https://direnv.net/), see [installation instructions](https://direnv.net/docs/installation.html)
3. [Visual Studio Code](https://code.visualstudio.com/) with [recommended extensions](https://code.visualstudio.com/docs/editor/extension-marketplace#_recommended-extensions)

> [!IMPORTANT]
> When opening the project for the first time, run `direnv allow` and `pnpm install`

> [!TIP]
> To see all available development commands, run `task`

## Conventional Commits

This project follows the [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/) specification. Please make yourself familiar with it before pushing commits to the repository. It is important to adhere to this specification, as it is used to automatically generate [Semantic Versioning 2.0.0](https://semver.org/) compatible versions and changelogs.

[Cocogitto](https://docs.cocogitto.io/) is present in the development environment to provide assistance with conventional commits. For example, [`cog commit`](https://docs.cocogitto.io/guide/#conventional-commits) can be used instead of `git commit` to ensure the correctness when committing to the repository.
