{
  description = "1Password secrets integration for NixOS";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { nixpkgs, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (system: let
      pkgs = import nixpkgs {
        inherit system;
        # Allow unfree packages for test dependencies
        config.allowUnfree = true;
      };

      src = import ./nix/source.nix { inherit pkgs; };

      buildOpnix = pkgs.buildGoModule {
        pname = "opnix";
        version = "0.1.1";
        inherit src;
        vendorHash = "sha256-rmwZue0X6o0q29ZVe9bWHBOxHVx/yiMJXHc4urooaHo=";
        subPackages = [ "cmd/opnix" ];
      };

      checks = import ./nix/checks.nix { inherit pkgs src; } // {
        build = buildOpnix;
      };
    in {
      devShells.default = import ./nix/devshell.nix { inherit pkgs buildOpnix; };
      packages.default = buildOpnix;
      inherit checks;
      formatter = pkgs.alejandra;
    }) // {
      nixosModules.default = import ./nix/module.nix;

      darwinModules.default = import ./nix/darwin-module.nix;

      # Add Home Manager module output
      homeManagerModules.default = import ./nix/hm-module.nix;

      overlays.default = final: prev: {
        opnix = import ./nix/package.nix { pkgs = final; };
      };
    };
}
