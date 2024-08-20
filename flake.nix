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
            name = "go";
            runtimeInputs = with pkgs; [dotenvx go];
            text = ''dotenvx run --convention=nextjs -- go "$@"'';
          };
        in
          pkgs.mkShell {
            NIX_PATH = ["nixpkgs=${inputs.nixpkgs}"];
            nativeBuildInputs =
              [
                goWrapper
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
