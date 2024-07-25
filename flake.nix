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
          pnpm = pkgs.writeShellApplication {
            name = "pnpm";
            runtimeInputs = with pkgs; [dotenvx pnpm_9];
            text = ''dotenvx run --env-file "''${ROOT_DOTENV:-/dev/null}" -- pnpm "$@"'';
          };
        in
          pkgs.mkShell {
            NIX_PATH = ["nixpkgs=${inputs.nixpkgs}"];

            shellHook = ''export ROOT_DOTENV=$(readlink -f .env)'';

            nativeBuildInputs =
              [pnpm]
              ++ (with pkgs; [
                # JS tools
                nodejs
                turbo

                # Nix tools
                alejandra
                nixd
              ]);
          };
      };
    };
}
