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
            nativeBuildInputs = with pkgs; [
              gh
              go
              go-task
              jq
              nodejs_latest
              pnpm
              protoc-gen-connect-go
              protoc-gen-go
              scripts
            ];
          };

        devShells.default = pkgs.mkShell {
          # Specify Nixpkgs path for improved nixd intellisense
          NIX_PATH = ["nixpkgs=${inputs.nixpkgs}"];

          # Use Electron binary from Nixpkgs in development for NixOS compatibility
          ELECTRON_SKIP_BINARY_DOWNLOAD = 1;
          ELECTRON_EXEC_PATH = "${pkgs.electron}/bin/electron";

          shellHook = ''
            # Export PNPM binaries into path for better DX
            export PATH=$PATH:$(pnpm bin)
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
