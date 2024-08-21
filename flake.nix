{
  inputs = {
    flake-parts.url = "github:hercules-ci/flake-parts";
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    systems.url = "github:nix-systems/default";

    # Follows
    flake-parts.inputs.nixpkgs-lib.follows = "nixpkgs";
  };

  outputs = inputs @ {flake-parts, ...}:
    flake-parts.lib.mkFlake {inherit inputs;} {
      systems = import inputs.systems;
      perSystem = {pkgs, ...}: {
        devShells.default = let
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
            NIX_PATH = ["nixpkgs=${inputs.nixpkgs}"];
            nativeBuildInputs =
              [
                goWrapper
                pnpmWrapper
              ]
              ++ (with pkgs; [
                alejandra
                dotenvx
                nixd
              ]);
          };
      };
    };
}
