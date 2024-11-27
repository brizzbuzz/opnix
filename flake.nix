{
  description = "1Password secrets integration for NixOS";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = {
    nixpkgs,
    flake-utils,
    ...
  }:
    flake-utils.lib.eachDefaultSystem (system: let
      pkgs = import nixpkgs {
        inherit system;
      };
    in {
      devShells.default = pkgs.mkShell {
        buildInputs = with pkgs; [
          alejandra
          just
          go
          gopls
          gotools
          go-tools
          golangci-lint
          nil
          # Add our built package to the shell
          (pkgs.buildGoModule {
            pname = "opnix";
            version = "0.1.0";
            src = ./.;
            vendorHash = "sha256-K8xgmXvhZ4PFUryu9/UsnwZ0Lohi586p1bzqBQBp1jo=";
            subPackages = [ "cmd/opnix" ];
          })
        ];
      };

      packages.default = import ./nix/package.nix { inherit pkgs; };

      formatter = pkgs.alejandra;
    }) // {
      nixosModules.default = import ./nix/module.nix;

      overlays.default = final: prev: {
        opnix = import ./nix/package.nix { pkgs = final; };
      };
    };
}
