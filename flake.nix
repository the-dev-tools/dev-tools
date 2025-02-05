{
  inputs = {
    flake-parts.url = "github:hercules-ci/flake-parts";
    gha-nix-develop.url = "github:nicknovitski/nix-develop";
    nixpkgs.url = "github:nixos/nixpkgs/nixos-24.11";
    systems.url = "github:nix-systems/default";

    # Follows
    flake-parts.inputs.nixpkgs-lib.follows = "nixpkgs";
    gha-nix-develop.inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs = inputs @ {flake-parts, ...}:
    flake-parts.lib.mkFlake {inherit inputs;} {
      systems = import inputs.systems;
      perSystem = {
        inputs',
        pkgs,
        self',
        ...
      }: {
        packages.gha-nix-develop = inputs'.gha-nix-develop.packages.default;

        devShells.runner = let
          scripts = pkgs.writeShellApplication {
            name = "scripts";
            runtimeInputs = with pkgs; [pnpm];
            runtimeEnv.NODE_OPTIONS = "--disable-warning=ExperimentalWarning";
            text = ''pnpm run --filter="*/scripts" cli "$@"'';
          };
        in
          pkgs.mkShell {
            shellHook = ''
              # Export Go and PNPM paths
              [ -n "$(go env GOBIN)" ] && export PATH="$(go env GOBIN):$PATH"
              [ -n "$(go env GOPATH)" ] && export PATH="$(go env GOPATH)/bin:$PATH"
              [ -n "$(pnpm bin)" ] && export PATH="$(pnpm bin):$PATH"
            '';

            nativeBuildInputs = with pkgs; [
              gh
              go
              go-task
              jq
              nodejs_latest
              pnpm
              scripts
            ];
          };

        devShells.default = pkgs.mkShell {
          inherit (self'.devShells.runner) shellHook;

          # Specify Nixpkgs path for improved nixd intellisense
          NIX_PATH = ["nixpkgs=${inputs.nixpkgs}"];

          # Use Electron binary from Nixpkgs in development for NixOS compatibility
          ELECTRON_SKIP_BINARY_DOWNLOAD = 1;
          ELECTRON_EXEC_PATH = "${pkgs.electron}/bin/electron";

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
