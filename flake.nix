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
      perSystem = {pkgs, ...}: let
        runnerBuildInputs = with pkgs; [
          # JS tools
          nodejs
          pnpm_9
        ];
      in {
        devShells.default = pkgs.mkShell {
          NIX_PATH = ["nixpkgs=${inputs.nixpkgs}"];

          nativeBuildInputs =
            runnerBuildInputs
            ++ (with pkgs; [
              # Nix tools
              alejandra
              nixd
            ]);
        };

        devShells.runner = pkgs.mkShell {
          nativeBuildInputs = runnerBuildInputs;
        };
      };
    };
}
