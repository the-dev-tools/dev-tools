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
                (dotenvx-wrapper (pkgs.pnpm_9))
                (dotenvx-wrapper (pkgs.turbo))
              ]
              ++ (with pkgs; [
                dotenvx
                go
                go-task
                jq
                nodejs_latest
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
