{
  pkgs,
  nixpkgs,
  system,
  src,
  vendorHash,
}:
{
  # Run tests
  go-tests = pkgs.buildGoModule {
    pname = "opnix-go-tests";
    version = "0.10.1";
    inherit src;
    inherit vendorHash;

    checkPhase = ''
      go test ./...
    '';

    installPhase = "touch $out";
  };

  # Run golangci-lint
  go-lint = pkgs.buildGoModule {
    pname = "opnix-go-lint";
    version = "0.10.1";
    inherit src;
    inherit vendorHash;

    nativeBuildInputs = [pkgs.golangci-lint];

    buildPhase = ''
      export GOLANGCI_LINT_CACHE=$TMPDIR/golangci-lint
      export XDG_CACHE_HOME=$TMPDIR/cache

      mkdir -p $GOLANGCI_LINT_CACHE
      mkdir -p $XDG_CACHE_HOME

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
        in "echo -n '${cfg}' >> .golangci.yaml"
      }

      golangci-lint run --allow-parallel-runners \
        --timeout=5m \
        --max-same-issues=20 \
        ./...
    '';

    installPhase = "touch $out";
  };

  # Check nix formatting
  nix-fmt-check =
    pkgs.runCommand "opnix-nix-fmt-check"
    {
      nativeBuildInputs = [pkgs.alejandra];
      inherit src;
    } ''
      cp -r $src/* .
      alejandra --check .
      touch $out
    '';
}
// pkgs.lib.optionalAttrs pkgs.stdenv.isLinux (let
  nixosConfig = polling:
    (nixpkgs.lib.nixosSystem {
      inherit system;
      modules = [
        ./module.nix
        {
          system.stateVersion = "26.05";
          services.onepassword-secrets = {
            enable = true;
            secrets.testSecret.reference = "op://Example/Service/password";
            systemdIntegration.polling = polling;
          };
        }
      ];
    }).config;

  defaultPollingConfig = nixosConfig {};
  enabledPollingConfig = nixosConfig {
    enable = true;
    interval = "45min";
  };
in {
  module-evaluation = assert !(defaultPollingConfig.systemd.services ? opnix-secrets-poll);
  assert !(defaultPollingConfig.systemd.timers ? opnix-secrets-poll);
  assert enabledPollingConfig.systemd.services.opnix-secrets-poll.serviceConfig.Type == "oneshot";
  assert !(enabledPollingConfig.systemd.services.opnix-secrets-poll.serviceConfig ? RemainAfterExit);
  assert nixpkgs.lib.hasInfix "systemctl stop opnix-secrets-watcher.path" enabledPollingConfig.systemd.services.opnix-secrets-poll.script;
  assert nixpkgs.lib.hasInfix "trap restore_watcher EXIT" enabledPollingConfig.systemd.services.opnix-secrets-poll.script;
  assert enabledPollingConfig.systemd.timers.opnix-secrets-poll.timerConfig.OnActiveSec == "45min";
  assert enabledPollingConfig.systemd.timers.opnix-secrets-poll.timerConfig.OnUnitActiveSec == "45min";
  assert enabledPollingConfig.systemd.timers.opnix-secrets-poll.timerConfig.Unit == "opnix-secrets-poll.service";
    pkgs.runCommand "opnix-module-evaluation" {} ''
      touch $out
    '';
})
