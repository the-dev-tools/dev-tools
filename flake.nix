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
      perSystem = {
        lib,
        pkgs,
        ...
      }: let
        package = lib.importJSON ./package.json;

        setupNodeModules = let
          inherit (package) version;
          pname = "${package.name}-node_modules";
          src = with lib.fileset;
            toSource {
              root = ./.;
              fileset = unions [./package.json ./pnpm-lock.yaml];
            };
          pnpmDeps = pkgs.pnpm_9.fetchDeps {
            inherit pname version src;
            hash = "sha256-zqa7lEbk/9QNFZIbYrAWfVFKpwGNq1umwobNgCf1alk=";
          };
          result = pkgs.stdenv.mkDerivation {
            inherit pname version src pnpmDeps;
            nativeBuildInputs = [pkgs.pnpm_9.configHook];
            installPhase = "cp --recursive . $out";
          };
        in ''
          cp --recursive --update=none ${result}/node_modules .
          chmod --recursive +w node_modules
        '';

        taskInputs = with pkgs; [
          # JS tools
          nodejs
          pnpm_9

          # Other
          cocogitto
          go-task
        ];
      in {
        devShells.default = pkgs.mkShell {
          NIX_PATH = ["nixpkgs=${inputs.nixpkgs}"];

          nativeBuildInputs =
            taskInputs
            ++ (with pkgs; [
              # Nix tools
              alejandra
              nixd
            ]);
        };

        devShells.runner = pkgs.mkShell {
          nativeBuildInputs = taskInputs;
          shellHook = setupNodeModules;
        };
      };
    };
}
