{ pkgs, buildOpnix }:
pkgs.mkShell {
  buildInputs = with pkgs; [
    alejandra
    just
    go
    gopls
    gotools
    go-tools
    golangci-lint
    nil
    buildOpnix
  ];
}
