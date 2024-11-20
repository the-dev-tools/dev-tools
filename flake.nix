{
  inputs = {
    flake-parts.url = "github:hercules-ci/flake-parts";
    gha-nix-develop.url = "github:nicknovitski/nix-develop";
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
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
          dotenvx-wrapper = pkg:
            pkgs.writeShellApplication {
              name = pkg.pname;
              runtimeInputs = [pkgs.dotenvx pkg];
              text = ''
                dotenvx run \
                  --log-level "''${DOTENV_LOG_LEVEL:-info}" \
                  --convention=nextjs \
                  -- ${pkg.pname} "$@"
              '';
            };
        in
          pkgs.mkShell {
            nativeBuildInputs =
              [
                ## TODO: fix this broke the gopls ( go's lsp ) (dotenvx-wrapper (pkgs.go))
                (dotenvx-wrapper (pkgs.pnpm_9))
                pkgs.gopls
              ]
              ++ (with pkgs; [
                nodejs_latest

                # build tools
                dotenvx
                go-task
                jq

                # cross-compilation dependencies
                pkgsCross.mingw32.buildPackages.gcc
                pkgsCross.mingwW64.buildPackages.gcc
                pkgsCross.aarch64-multiplatform.buildPackages.gcc
              ]);
          };

        devShells.default = pkgs.mkShell {
          NIX_PATH = ["nixpkgs=${inputs.nixpkgs}"];
          nativeBuildInputs =
            self'.devShells.runner.nativeBuildInputs
            ++ (with pkgs; [
              alejandra
              nixd
            ]);
        };
      };
    };
}
