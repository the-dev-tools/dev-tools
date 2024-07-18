# Changelog
All notable changes to this project will be documented in this file. See [conventional commits](https://www.conventionalcommits.org/) for commit guidelines.

- - -
## [0.4.0](https://github.com/DevToolsGit/api-recorder/compare/9b37c6ed545a31e8bb919b21996b51df92f50099..0.4.0) - 2024-07-18
#### Bug Fixes
- make postman request item keys unique - ([d7d13e6](https://github.com/DevToolsGit/api-recorder/commit/d7d13e6fe6c436d0446f252a3908e847296fa2ab)) - [@Tomaszal](https://github.com/Tomaszal)
- stop recording on unaccessible pages - ([4853a00](https://github.com/DevToolsGit/api-recorder/commit/4853a00db6511f09bd5b4c2b2ad8ab2a53cf8558)) - [@Tomaszal](https://github.com/Tomaszal)
- stop recording on debugger detach - ([80b24f5](https://github.com/DevToolsGit/api-recorder/commit/80b24f50e3e05c2f0b0a286ada91b6460cfc801e)) - [@Tomaszal](https://github.com/Tomaszal)
- watch fetch resources in addition to xhr - ([a658620](https://github.com/DevToolsGit/api-recorder/commit/a6586207e3697d6d843b2537fb14643f01ec71d4)) - [@Tomaszal](https://github.com/Tomaszal)
#### Continuous Integration
- fix deployment artifact path - ([9b37c6e](https://github.com/DevToolsGit/api-recorder/commit/9b37c6ed545a31e8bb919b21996b51df92f50099)) - [@Tomaszal](https://github.com/Tomaszal)
#### Documentation
- add missing changelog entries - ([879f10b](https://github.com/DevToolsGit/api-recorder/commit/879f10be157136da9b9854ae2f57af5ca49f9995)) - [@Tomaszal](https://github.com/Tomaszal)
#### Features
- implement hostname blacklist - ([5ca631e](https://github.com/DevToolsGit/api-recorder/commit/5ca631e7103a7a07c218da67c01aec64105e6dad)) - [@Tomaszal](https://github.com/Tomaszal)

- - -

## [0.3.0](https://github.com/DevToolsGit/api-recorder/compare/1e456e42c4619719f34ef5f5d6895c8f7a8fcfbe..0.3.0) - 2024-07-17
#### Continuous Integration
- fix deployment workflow permissions - ([896957a](https://github.com/DevToolsGit/api-recorder/commit/896957a3cb221625843d8b4d52d79a828d2a10f6)) - [@Tomaszal](https://github.com/Tomaszal)
- fix git push tag command - ([8659b71](https://github.com/DevToolsGit/api-recorder/commit/8659b7140da6d041f3847a51f10a45c635138f83)) - [@Tomaszal](https://github.com/Tomaszal)
- fix bump command - ([72b0b2a](https://github.com/DevToolsGit/api-recorder/commit/72b0b2a2c2b7071a339251dca2511a3e002b6038)) - [@Tomaszal](https://github.com/Tomaszal)
- pre-cache runner dev shell - ([bcecf71](https://github.com/DevToolsGit/api-recorder/commit/bcecf718e6b476b13c9803cce2a67a743b1a135e)) - [@Tomaszal](https://github.com/Tomaszal)
- improve workflow and job naming - ([ac88472](https://github.com/DevToolsGit/api-recorder/commit/ac88472e604a1032319418f889556a8ee6a58d25)) - [@Tomaszal](https://github.com/Tomaszal)
- implement cd - ([1e456e4](https://github.com/DevToolsGit/api-recorder/commit/1e456e42c4619719f34ef5f5d6895c8f7a8fcfbe)) - [@Tomaszal](https://github.com/Tomaszal)
#### Features
- improve authentication confirmation flow - ([2fd24a0](https://github.com/DevToolsGit/api-recorder/commit/2fd24a0878c79999d4893c50523e9e2cd2c53639)) - [@Tomaszal](https://github.com/Tomaszal)

- - -

## [0.2.0](https://github.com/DevToolsGit/api-recorder/compare/f5b29e52534d5ffbedd4319cf4e18972b2149f77..0.2.0) - 2024-07-17
#### Bug Fixes
- dependency issues - ([b287975](https://github.com/DevToolsGit/api-recorder/commit/b287975c2c023fae39e9c73ab709f87a49be7e15)) - [@Tomaszal](https://github.com/Tomaszal)
- move tokens.json to global gitignore so it is ignored by prettier too - ([1ae5b00](https://github.com/DevToolsGit/api-recorder/commit/1ae5b00f4eb9e16244662efa3311568ac746bc73)) - [@Tomaszal](https://github.com/Tomaszal)
- production build error - ([6b155e8](https://github.com/DevToolsGit/api-recorder/commit/6b155e8ae14d536dea4aec41209d115cb636b192)) - [@Tomaszal](https://github.com/Tomaszal)
- focus ring border render glitches - ([b5e52d5](https://github.com/DevToolsGit/api-recorder/commit/b5e52d5dc8ff20dc24f63092fc57dad2722c4b7d)) - [@Tomaszal](https://github.com/Tomaszal)
- vertically center api call items - ([c5efeda](https://github.com/DevToolsGit/api-recorder/commit/c5efedadb322100d69afcd26841468ab483b7215)) - [@Tomaszal](https://github.com/Tomaszal)
- remove local storage size limit - ([b6ed820](https://github.com/DevToolsGit/api-recorder/commit/b6ed8208d81cdda5d9974b9cdb85d68d875ed5b0)) - [@Tomaszal](https://github.com/Tomaszal)
- eslint-plugin-tailwindcss config to detect all sources - ([d6b92ec](https://github.com/DevToolsGit/api-recorder/commit/d6b92ec1a8c14067f3f583c5bcbca20459b51a82)) - [@Tomaszal](https://github.com/Tomaszal)
- response race condition by moving collection state to service worker and syncing periodically - ([8afaf41](https://github.com/DevToolsGit/api-recorder/commit/8afaf41c320980293921d7bfb28d021dc00eaf9e)) - [@Tomaszal](https://github.com/Tomaszal)
- add search label - ([d2e7a66](https://github.com/DevToolsGit/api-recorder/commit/d2e7a66d3c1b9d9dc775a36a985c1ba61d4f6534)) - [@Tomaszal](https://github.com/Tomaszal)
- lint warning - ([b1bd510](https://github.com/DevToolsGit/api-recorder/commit/b1bd51069bf2fa458c648bf7187b7324eea8d6d0)) - [@Tomaszal](https://github.com/Tomaszal)
- re-enable eslint-plugin-import-x with upstream fixes - ([16f6738](https://github.com/DevToolsGit/api-recorder/commit/16f6738f40e266eb1b4595e0b68fff16ddbb0e67)) - [@Tomaszal](https://github.com/Tomaszal)
- narrow typescript file search - ([7f51493](https://github.com/DevToolsGit/api-recorder/commit/7f51493c4d4587b1854be47fa3aba38714647544)) - [@Tomaszal](https://github.com/Tomaszal)
- improve variable namings - ([ab0338b](https://github.com/DevToolsGit/api-recorder/commit/ab0338b1ea55de2f9387c630848327b7957c1e2e)) - [@Tomaszal](https://github.com/Tomaszal)
- disable eslint-plugin-import-x until fixed upstream - ([5af0ce5](https://github.com/DevToolsGit/api-recorder/commit/5af0ce51caa7347ec5f224ebfeb24a75e4c627bc)) - [@Tomaszal](https://github.com/Tomaszal)
- eslint not working in vscode environment - ([bce7a5f](https://github.com/DevToolsGit/api-recorder/commit/bce7a5f459d215e98835ccb42e58c63a20dcc64b)) - [@Tomaszal](https://github.com/Tomaszal)
#### Continuous Integration
- move dependencies out of nix store into a runner dev shell - ([c04c207](https://github.com/DevToolsGit/api-recorder/commit/c04c20769b276b48ddc60d2481526c285978ec13)) - [@Tomaszal](https://github.com/Tomaszal)
- update actions - ([21b2bfd](https://github.com/DevToolsGit/api-recorder/commit/21b2bfd44a9e2d3dc938bbbf3ffbdd8cc5786463)) - [@Tomaszal](https://github.com/Tomaszal)
- add build log to flake check - ([c953ddd](https://github.com/DevToolsGit/api-recorder/commit/c953ddd7828df40b539f1ee6a193045a6da4614a)) - [@Tomaszal](https://github.com/Tomaszal)
- implement ci checks - ([84bb6d7](https://github.com/DevToolsGit/api-recorder/commit/84bb6d76c2b2e2da322b731964e9ade6884d268b)) - [@Tomaszal](https://github.com/Tomaszal)
#### Documentation
- improve taskfile descriptions - ([736a1d9](https://github.com/DevToolsGit/api-recorder/commit/736a1d9342b0d6e6dfd52c2e5c1d5e27cffff22e)) - [@Tomaszal](https://github.com/Tomaszal)
- move design token docs to taskfile - ([4361313](https://github.com/DevToolsGit/api-recorder/commit/4361313823b84662e4f8c2d787184bd50f17871a)) - [@Tomaszal](https://github.com/Tomaszal)
- add readme with starting instructions - ([9c1aee7](https://github.com/DevToolsGit/api-recorder/commit/9c1aee7108fe0730fd1b3180c0bf816db0c2cd8d)) - [@Tomaszal](https://github.com/Tomaszal)
#### Features
- setup cocogitto for conventional commit and semver enforcement - ([e3a888a](https://github.com/DevToolsGit/api-recorder/commit/e3a888af518d13a81774e7aec68ed1093325b26a)) - [@Tomaszal](https://github.com/Tomaszal)
- enable dev env on any all nix compatible systems - ([8d9e304](https://github.com/DevToolsGit/api-recorder/commit/8d9e30434821dfe274721ce7dc938564540e78e9)) - [@Tomaszal](https://github.com/Tomaszal)
- generate token studio config from tailwind config - ([1ae8a74](https://github.com/DevToolsGit/api-recorder/commit/1ae8a74cdaf8b6022bc3fe7acbf9cea02f827eb4)) - [@Tomaszal](https://github.com/Tomaszal)
- map recorded body & headers to postman request & response - ([a7f868d](https://github.com/DevToolsGit/api-recorder/commit/a7f868db2d840ebacf2aeb4b2c15eee0108809e4)) - [@Tomaszal](https://github.com/Tomaszal)
- deduplicate origin pathnames - ([66243d6](https://github.com/DevToolsGit/api-recorder/commit/66243d62e1cceab4f7fbe15ee435a3a108b100f3)) - [@Tomaszal](https://github.com/Tomaszal)
- add request url tooltip - ([72716ae](https://github.com/DevToolsGit/api-recorder/commit/72716aeb7f2f1121db997c985cf45841018b46d1)) - [@Tomaszal](https://github.com/Tomaszal)
- persist selection state - ([99336a7](https://github.com/DevToolsGit/api-recorder/commit/99336a74c739778ac3abf5ea02b8868ab60cc761)) - [@Tomaszal](https://github.com/Tomaszal)
- add extension icon - ([cd2d0af](https://github.com/DevToolsGit/api-recorder/commit/cd2d0afe0c77bf3b3ec0e558fb65c29f5403e956)) - [@Tomaszal](https://github.com/Tomaszal)
- reverse recording order to show most recent items on top - ([923af5b](https://github.com/DevToolsGit/api-recorder/commit/923af5bfd5d27f7d9c02267ebcd9b59700ec3e59)) - [@Tomaszal](https://github.com/Tomaszal)
- add missing icons - ([0ac3e9a](https://github.com/DevToolsGit/api-recorder/commit/0ac3e9a64bb9d7d2533543fed19b9a1202118733)) - [@Tomaszal](https://github.com/Tomaszal)
- improve login flow - ([cdd63c5](https://github.com/DevToolsGit/api-recorder/commit/cdd63c5c81b1729dd7d79a8450c598adb5eae18f)) - [@Tomaszal](https://github.com/Tomaszal)
- increase recording indicator icon size - ([0b5d3a0](https://github.com/DevToolsGit/api-recorder/commit/0b5d3a0745fdb90fa379370afc7d73ae05bbb09b)) - [@Tomaszal](https://github.com/Tomaszal)
- add api calls list empty state - ([2c56ee1](https://github.com/DevToolsGit/api-recorder/commit/2c56ee132f7dac4db45893b8a5be881b50727c88)) - [@Tomaszal](https://github.com/Tomaszal)
- add intro page - ([2f6b867](https://github.com/DevToolsGit/api-recorder/commit/2f6b867ba72fd90e43e3d1030030c67b69f5c82b)) - [@Tomaszal](https://github.com/Tomaszal)
- improve recorder page layout - ([33f9519](https://github.com/DevToolsGit/api-recorder/commit/33f95197f19a409c34c679a6d1c43d07a5fd055a)) - [@Tomaszal](https://github.com/Tomaszal)
- add request timestamp - ([53641e7](https://github.com/DevToolsGit/api-recorder/commit/53641e7e8aa29cab01243d67de49c87dfd8b1333)) - [@Tomaszal](https://github.com/Tomaszal)
- improve request url ui - ([19e2149](https://github.com/DevToolsGit/api-recorder/commit/19e214944b6df92031b6e3a198d8f3c2c3cd68ab)) - [@Tomaszal](https://github.com/Tomaszal)
- implement authentication using magic link - ([95bc58e](https://github.com/DevToolsGit/api-recorder/commit/95bc58ea10ab820f2407469a0a23e381780506c7)) - [@Tomaszal](https://github.com/Tomaszal)
- implement search and improve collection selection logic - ([f7c1eaa](https://github.com/DevToolsGit/api-recorder/commit/f7c1eaacc52693d346f45f1972eb418370db900f)) - [@Tomaszal](https://github.com/Tomaszal)
- add styled focus ring - ([72b0c4a](https://github.com/DevToolsGit/api-recorder/commit/72b0c4a3cf970433894771b3db88114f7bb556f8)) - [@Tomaszal](https://github.com/Tomaszal)
- add template tw utility for plugin hinting - ([03d7ae3](https://github.com/DevToolsGit/api-recorder/commit/03d7ae3568f9808dcd74b21b1c3406303a72e173)) - [@Tomaszal](https://github.com/Tomaszal)
- background image effects - ([7b3d399](https://github.com/DevToolsGit/api-recorder/commit/7b3d399e27ad61459195d9b0bbedf49188f0bdd0)) - [@Tomaszal](https://github.com/Tomaszal)
- postman collection export - ([dc5ba1c](https://github.com/DevToolsGit/api-recorder/commit/dc5ba1c93e64021b9589df1747116190ea58d721)) - [@Tomaszal](https://github.com/Tomaszal)
- request selection for export - ([f0c7786](https://github.com/DevToolsGit/api-recorder/commit/f0c7786f43ba147fa4ea756dd25df1607e6fd572)) - [@Tomaszal](https://github.com/Tomaszal)
- host selection - ([19d378b](https://github.com/DevToolsGit/api-recorder/commit/19d378b65b96782ed7442a89cc1a4d2088841d75)) - [@Tomaszal](https://github.com/Tomaszal)
- improve host list and selection with rac - ([a5c9ad8](https://github.com/DevToolsGit/api-recorder/commit/a5c9ad8a19ece358d5739d28a483050af1fdd198)) - [@Tomaszal](https://github.com/Tomaszal)
- recording and call number indicators - ([d064c38](https://github.com/DevToolsGit/api-recorder/commit/d064c38cb7207b600ef516e4176ed98f705987fe)) - [@Tomaszal](https://github.com/Tomaszal)
- setup font - ([cef9952](https://github.com/DevToolsGit/api-recorder/commit/cef9952d0b0876c807e5ff5ce4f94c4ff7169025)) - [@Tomaszal](https://github.com/Tomaszal)
- add background image - ([c69c944](https://github.com/DevToolsGit/api-recorder/commit/c69c9441f3f6099a27f2312126a074bfebe178e8)) - [@Tomaszal](https://github.com/Tomaszal)
- ui system foundation and button component - ([bd2e0f6](https://github.com/DevToolsGit/api-recorder/commit/bd2e0f65e07d8fcf20cfee83693aff46d319bf20)) - [@Tomaszal](https://github.com/Tomaszal)
- implement basic ui layout - ([c8c6b62](https://github.com/DevToolsGit/api-recorder/commit/c8c6b620793903cec6c7ca3a84ed18bdad1aab4d)) - [@Tomaszal](https://github.com/Tomaszal)
- implement request recording into postman collection - ([ffe81aa](https://github.com/DevToolsGit/api-recorder/commit/ffe81aa44b82784c6adebe78b0f222e90c080689)) - [@Tomaszal](https://github.com/Tomaszal)
- add reset functionality - ([dcf4ccf](https://github.com/DevToolsGit/api-recorder/commit/dcf4ccfb07a382562f5a5b741d33d6eda6454f35)) - [@Tomaszal](https://github.com/Tomaszal)
- add display name - ([879ed7e](https://github.com/DevToolsGit/api-recorder/commit/879ed7e5d3f998239a533763dea62ec06fecb5b3)) - [@Tomaszal](https://github.com/Tomaszal)
- implement tab navigation recording into a postman collection - ([3b607e5](https://github.com/DevToolsGit/api-recorder/commit/3b607e5e94ee8d90d910b493dd4932332e2ae424)) - [@Tomaszal](https://github.com/Tomaszal)
- add postman schema - ([0006339](https://github.com/DevToolsGit/api-recorder/commit/0006339d0a2acf79a7a313a97d59a116377f8a77)) - [@Tomaszal](https://github.com/Tomaszal)
- add editorconfig - ([eafd796](https://github.com/DevToolsGit/api-recorder/commit/eafd79637bc65dca25a54cf9b3be6c99eab589ee)) - [@Tomaszal](https://github.com/Tomaszal)
- use debugger api to collect all possible data - ([95f159e](https://github.com/DevToolsGit/api-recorder/commit/95f159e8fcf2e03ae258ee29bcc0911df006de00)) - [@Tomaszal](https://github.com/Tomaszal)
- simplify the ui to the most basic form until final design is established - ([22afbf0](https://github.com/DevToolsGit/api-recorder/commit/22afbf01a017d320d3dc6f8ba92c9cc27da2c2ae)) - [@Tomaszal](https://github.com/Tomaszal)
- implement basic network call recording - ([8f3c4b3](https://github.com/DevToolsGit/api-recorder/commit/8f3c4b33b9a49b4a2728ddd207e5fb2114b973c8)) - [@Tomaszal](https://github.com/Tomaszal)
- implement linting - ([fa01d69](https://github.com/DevToolsGit/api-recorder/commit/fa01d69b1bb1e81ab8b6b4ef6f316d18d0407a40)) - [@Tomaszal](https://github.com/Tomaszal)
- add strictest typescript config preset - ([cdf4af3](https://github.com/DevToolsGit/api-recorder/commit/cdf4af37104d579491bf49e6644c00388dd7bc08)) - [@Tomaszal](https://github.com/Tomaszal)
- setup tailwind - ([841c120](https://github.com/DevToolsGit/api-recorder/commit/841c12086a87cde4351401cec3f55122116f9f9a)) - [@Tomaszal](https://github.com/Tomaszal)
- add taskfile for improved task management - ([e93ca6e](https://github.com/DevToolsGit/api-recorder/commit/e93ca6ec0548cadf3afcb1969681702ec7666008)) - [@Tomaszal](https://github.com/Tomaszal)
- add nix flake for reproducible dev shell - ([962a1a8](https://github.com/DevToolsGit/api-recorder/commit/962a1a8d3b6d3774ff4b6baa7abc421aa37c9281)) - [@Tomaszal](https://github.com/Tomaszal)
- add vscode configs - ([f5b29e5](https://github.com/DevToolsGit/api-recorder/commit/f5b29e52534d5ffbedd4319cf4e18972b2149f77)) - [@Tomaszal](https://github.com/Tomaszal)
#### Miscellaneous Chores
- bump version to correctly reflect chrome web store - ([e5e750c](https://github.com/DevToolsGit/api-recorder/commit/e5e750c90ffa8b443ef653a2abbecb0dcf6b291c)) - [@Tomaszal](https://github.com/Tomaszal)
- improve tailwind vscode integration - ([e2ffc1e](https://github.com/DevToolsGit/api-recorder/commit/e2ffc1e049f739a45ee3f7b08263530fb3784365)) - [@Tomaszal](https://github.com/Tomaszal)
#### Refactoring
- move illustrations to a ui module - ([7242ca5](https://github.com/DevToolsGit/api-recorder/commit/7242ca528ebf67292c5af534422c092c245af02e)) - [@Tomaszal](https://github.com/Tomaszal)

- - -

## [0.1.0](https://github.com/DevToolsGit/api-recorder/compare/e262aaec6e697044078b664b7f8d5071e1f66b79..0.1.0) - 2024-07-17
#### Features
- init plasmo project - ([e262aae](https://github.com/DevToolsGit/api-recorder/commit/e262aaec6e697044078b664b7f8d5071e1f66b79)) - [@Tomaszal](https://github.com/Tomaszal)

- - -

Changelog generated by [cocogitto](https://github.com/cocogitto/cocogitto).
