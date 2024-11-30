{ pkgs }:
pkgs.lib.cleanSourceWith {
  src = ../.;
  filter = path: type:
    pkgs.lib.cleanSourceFilter path type && 
    (
      builtins.match ".*\\.go$" path != null ||
      builtins.match ".*go\\.(mod|sum)$" path != null ||
      builtins.match ".*/cmd(/.*)?$" path != null ||
      builtins.match ".*/internal(/.*)?$" path != null
    );
}
