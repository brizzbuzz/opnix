{ pkgs, src }:

let
  darwinFrameworks =
    if pkgs.stdenv.isDarwin then [
      pkgs.darwin.apple_sdk.frameworks.CoreFoundation
      pkgs.darwin.apple_sdk.frameworks.Security
    ] else [ ];
in
{
  # Run tests
  go-tests = pkgs.stdenv.mkDerivation {
    name = "opnix-go-tests";
    inherit src;

    nativeBuildInputs = [ pkgs.go ];
    buildInputs = darwinFrameworks;

    # Required for darwin frameworks
    NIX_LDFLAGS =
      if pkgs.stdenv.isDarwin then
        "-F${pkgs.darwin.apple_sdk.frameworks.CoreFoundation}/Library/Frameworks -framework CoreFoundation " +
        "-F${pkgs.darwin.apple_sdk.frameworks.Security}/Library/Frameworks -framework Security"
      else "";

    buildPhase = ''
      # Set up Go environment
      export GOPATH=$TMPDIR/go
      export GOCACHE=$TMPDIR/go-cache
      export GO111MODULE=on

      # Create a clean project directory
      mkdir -p $TMPDIR/workspace
      cd $TMPDIR/workspace

      # Copy source files
      cp -r $src/* .

      # Initialize and verify modules
      go mod download

      # Run tests
      go test ./...
    '';

    installPhase = "touch $out";
  };

  # Run golangci-lint
  go-lint = pkgs.stdenv.mkDerivation {
    name = "opnix-go-lint";
    inherit src;

    nativeBuildInputs = [ pkgs.go pkgs.golangci-lint ];
    buildInputs = darwinFrameworks;

    # Required for darwin frameworks
    NIX_LDFLAGS =
      if pkgs.stdenv.isDarwin then
        "-F${pkgs.darwin.apple_sdk.frameworks.CoreFoundation}/Library/Frameworks -framework CoreFoundation " +
        "-F${pkgs.darwin.apple_sdk.frameworks.Security}/Library/Frameworks -framework Security"
      else "";

    buildPhase =
      ''
                # Set up Go environment
                export GOPATH=$TMPDIR/go
                export GOCACHE=$TMPDIR/go-cache
                export GO111MODULE=on
                export GOLANGCI_LINT_CACHE=$TMPDIR/golangci-lint
                export XDG_CACHE_HOME=$TMPDIR/cache

                # Create all necessary directories
                mkdir -p $GOLANGCI_LINT_CACHE
                mkdir -p $XDG_CACHE_HOME
                mkdir -p $GOCACHE
                mkdir -p $GOPATH

                # Create and move to workspace
                mkdir -p $TMPDIR/workspace
                cd $TMPDIR/workspace

                # Copy source files
                cp -r $src/* .

                ${
                  let
                    cfg = ''
        version: "2"
        linters:
          default: standard
          settings:
            errcheck:
              exclude-functions:
                - fmt.Fprintf
          exclusions:
            rules:
              - path: ".*_test\\.go$"
                linters:
                  - errcheck
                    '';
                  in
                    "echo -n '${cfg}' >> .golangci.yaml"
                }

                # Initialize modules
                go mod download

                # Run linter
                golangci-lint run --allow-parallel-runners \
                  --timeout=5m \
                  --max-same-issues=20 \
                  ./...
      '';

    installPhase = "touch $out";
  };

  # Check nix formatting
  nix-fmt-check = pkgs.runCommand "opnix-nix-fmt-check"
    {
      nativeBuildInputs = [ pkgs.alejandra ];
      inherit src;
    } ''
    cp -r $src/* .
    alejandra --check .
    touch $out
  '';
}
