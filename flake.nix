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
        inherit (package) version;
        pname = package.name;
        src = ./.;

        taskInputs = with pkgs; [
          # JS tools
          nodejs
          pnpm_9

          # Other
          cocogitto
          go-task
        ];
      in {
        legacyPackages = pkgs;

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

        checks.lint = pkgs.stdenv.mkDerivation {
          inherit version src;
          pname = "${pname}-check-lint";
          pnpmDeps = pkgs.pnpm_9.fetchDeps {
            inherit pname version src;
            hash = "sha256-zqa7lEbk/9QNFZIbYrAWfVFKpwGNq1umwobNgCf1alk=";
          };
          nativeBuildInputs = taskInputs ++ (with pkgs; [pnpm_9.configHook]);
          doCheck = true;
          checkPhase = "task lint COG=false";
          installPhase = "touch $out";
        };
      };
    };
}
