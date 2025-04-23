{
  description = "Exa ansible repo main flake";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      supportedSystems =
        [ "x86_64-linux" "aarch64-darwin" "x86_64-darwin" "aarch64-linux" ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
      mkShell = { system }:
        let
          pkgs = import nixpkgs {
            inherit system;
            config = {
              allowUnfree = true;
            };
          };
        in
        pkgs.mkShell {
          buildInputs = with pkgs; [
            go
          ];
          shellHook = '''';
        };
    in
    {
      devShells =
        forAllSystems (system: { default = mkShell { inherit system; }; });
    };
}
