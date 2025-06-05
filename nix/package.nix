{ pkgs }:
pkgs.buildGoModule {
  pname = "opnix";
  version = "0.1.1";
  src = ../.;
  vendorHash = "sha256-rmwZue0X6o0q29ZVe9bWHBOxHVx/yiMJXHc4urooaHo=";
  subPackages = [ "cmd/opnix" ];
}
