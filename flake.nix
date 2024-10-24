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
          goWrapper = pkgs.writeShellApplication {
            name = "go run";
            runtimeInputs = with pkgs; [dotenvx go];
            text = ''dotenvx run --convention=nextjs -- go run "$@"'';
          };
          pnpmWrapper = pkgs.writeShellApplication {
            name = "pnpm";
            runtimeInputs = with pkgs; [dotenvx pnpm_9];
            text = ''dotenvx run --convention=nextjs -- pnpm "$@"'';
          };
        in
          pkgs.mkShell {
            nativeBuildInputs =
              [
                goWrapper
                pnpmWrapper
              ]
              ++ (with pkgs; [
                dotenvx
                go
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
