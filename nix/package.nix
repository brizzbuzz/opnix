{pkgs}:
pkgs.buildGoModule {
  pname = "opnix";
  version = "0.9.0";
  src = ../.;
  vendorHash = "sha256-H1v3SmLSrKgIUJInloLrFKTECddhZtBomFyIb8aqFzk=";
  subPackages = ["cmd/opnix"];
}
