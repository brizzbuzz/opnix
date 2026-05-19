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
        # Allow unfree packages for test dependencies
        config.allowUnfree = true;
      };

      src = import ./nix/source.nix {inherit pkgs;};
      vendorHash = "sha256-H1v3SmLSrKgIUJInloLrFKTECddhZtBomFyIb8aqFzk=";

      buildOpnix = pkgs.buildGoModule {
        pname = "opnix";
        version = "0.10.1";
        inherit src;
        inherit vendorHash;
        subPackages = ["cmd/opnix"];
      };

      checks =
        import ./nix/checks.nix {inherit pkgs src vendorHash;}
        // {
          build = buildOpnix;
        };
    in {
      packages.default = buildOpnix;
      inherit checks;
      devShells.default = import ./nix/devshell.nix {inherit pkgs buildOpnix;};
      formatter = pkgs.alejandra;
    })
    // {
      nixosModules.default = import ./nix/module.nix;

      darwinModules.default = import ./nix/darwin-module.nix;

      # Add Home Manager module output
      homeManagerModules.default = import ./nix/hm-module.nix;

      overlays.default = final: prev: {
        opnix = import ./nix/package.nix {pkgs = final;};
      };
    };
}
