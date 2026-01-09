{
  inputs = {
    cache-nix-action.url = "github:nix-community/cache-nix-action";
    flake-parts.url = "github:hercules-ci/flake-parts";
    gha-nix-develop.url = "github:nicknovitski/nix-develop";
    # Switch back to unstable once this PR lands
    # https://github.com/NixOS/nixpkgs/pull/465669
    # https://github.com/NixOS/nixpkgs/issues/376958
    nixpkgs.url = "github:nixos/nixpkgs/nixos-25.05";
    systems.url = "github:nix-systems/default";

    # Follows
    flake-parts.inputs.nixpkgs-lib.follows = "nixpkgs";
    gha-nix-develop.inputs.nixpkgs.follows = "nixpkgs";

    # Meta
    cache-nix-action.flake = false;
  };

  outputs = inputs @ {flake-parts, ...}:
    flake-parts.lib.mkFlake {inherit inputs;} {
      systems = import inputs.systems;
      perSystem = {
        config,
        inputs',
        pkgs,
        self',
        ...
      }: {
        packages.gha-nix-develop = inputs'.gha-nix-develop.packages.default;

        packages.gha-save-from-gc =
          (import "${inputs.cache-nix-action}/saveFromGC.nix" {
            inherit pkgs inputs;
            derivations = [config.devShells.runner];
          }).package;

        devShells.runner = let
          gha-scripts = pkgs.writeShellApplication {
            name = "gha-scripts";
            runtimeInputs = with pkgs; [pnpm jq];
            runtimeEnv.NODE_OPTIONS = "--disable-warning=ExperimentalWarning";
            text = ''pnpm run --filter="*/gha-scripts" cli "$@"'';
          };
        in
          pkgs.mkShell {
            shellHook = let
              export = {
                path,
                check ? path,
              }: ''
                [ -n "${check}" ] && mkdir --parent "${path}" && export PATH="${path}:$PATH"
              '';
            in ''
              # Export Go and PNPM paths
              ${export {path = "$(go env GOBIN)";}}
              ${export {
                path = "$(go env GOPATH)/bin";
                check = "$(go env GOPATH)";
              }}
              ${export {path = "$(pnpm bin)";}}
            '';

            nativeBuildInputs = with pkgs; [
              gcc
              gh
              gha-scripts
              go_1_25
              go-task
              jq
              nodejs_latest
              pnpm
              protoc-gen-connect-go
            ];
          };

        devShells.default = pkgs.mkShell {
          # Specify Nixpkgs path for improved nixd intellisense
          NIX_PATH = ["nixpkgs=${inputs.nixpkgs}"];

          # Use Electron binary from Nixpkgs in development for NixOS compatibility
          ELECTRON_SKIP_BINARY_DOWNLOAD = 1;
          ELECTRON_EXEC_PATH = "${pkgs.electron}/bin/electron";

          shellHook = ''
            ${self'.devShells.runner.shellHook}

            if [ -n "''${GEMINI_CLI-}" ]; then
              export NX_TUI=false
              export TASK_OUTPUT=prefixed
            fi
          '';

          nativeBuildInputs =
            self'.devShells.runner.nativeBuildInputs
            ++ (with pkgs; [
              alejandra
              gopls
              nixd
            ]);
        };
      };
    };
}
