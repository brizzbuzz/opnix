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
    version = "0.11.0";
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
    version = "0.11.0";
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
// (let
  lib = pkgs.lib;
  hmLib = lib.extend (_: _: {
    hm.dag = {
      entryBefore = _: value: value;
      entryAfter = _: value: value;
    };
  });
  hmConfig =
    (hmLib.evalModules {
      specialArgs = {inherit pkgs;};
      modules = [
        ./hm-module.nix
        {
          options = {
            home.homeDirectory = lib.mkOption {type = lib.types.str;};
            home.username = lib.mkOption {type = lib.types.str;};
            home.packages = lib.mkOption {
              type = lib.types.listOf lib.types.package;
              default = [];
            };
            home.activation = lib.mkOption {
              type = lib.types.attrsOf lib.types.anything;
              default = {};
            };
            assertions = lib.mkOption {
              type = lib.types.listOf lib.types.anything;
              default = [];
            };
          };
          config = {
            home.homeDirectory = "/home/opnix-test";
            home.username = "opnix-test";
            programs.onepassword-secrets = {
              enable = true;
              secrets = {
                defaultSecret.reference = "op://Example/Service/password";
                fileSecret = {
                  reference = "op://Example/Document/archive.bin";
                  kind = "file";
                };
              };
              templates.appConfig = {
                path = ".config/app.conf";
                content = "password=${hmConfig.programs.onepassword-secrets.placeholder.defaultSecret}";
              };
            };
          };
        }
      ];
    }).config;
  darwinConfig =
    (lib.evalModules {
      specialArgs = {inherit pkgs;};
      modules = [
        ./darwin-module.nix
        {
          options = {
            assertions = lib.mkOption {
              type = lib.types.listOf lib.types.anything;
              default = [];
            };
            users.knownGroups = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [];
            };
            users.groups = lib.mkOption {
              type = lib.types.attrsOf lib.types.anything;
              default = {};
            };
            users.users = lib.mkOption {
              type = lib.types.attrsOf lib.types.anything;
              default = {};
            };
            launchd.daemons = lib.mkOption {
              type = lib.types.attrsOf lib.types.anything;
              default = {};
            };
            system.activationScripts.extraActivation.text = lib.mkOption {
              type = lib.types.str;
              default = "";
            };
          };
          config.services.onepassword-secrets = {
            enable = true;
            secrets = {
              defaultSecret.reference = "op://Example/Service/password";
              fileSecret = {
                reference = "op://Example/Document/archive.bin";
                kind = "file";
              };
            };
            templates.appConfig = {
              path = "app.conf";
              content = "password=${darwinConfig.services.onepassword-secrets.placeholder.defaultSecret}";
              services = ["com.example.app"];
            };
          };
        }
      ];
    }).config;
in {
  hm-module-evaluation = assert !(hmConfig.programs.onepassword-secrets.placeholder ? fileSecret);
  assert hmConfig.programs.onepassword-secrets.placeholder.defaultSecret == "<OPNIX:7ab73f276766e0ae10ed5dbd2bad50a3db7dda14a35a05a91df135dd3769adfa>";
  assert hmConfig.programs.onepassword-secrets.templatePaths.appConfig == "/home/opnix-test/.config/app.conf";
  assert hmConfig.programs.onepassword-secrets.secrets.defaultSecret.kind == "field";
  assert hmConfig.programs.onepassword-secrets.secrets.fileSecret.kind == "file";
    pkgs.runCommand "opnix-hm-module-evaluation" {
      nativeBuildInputs = [pkgs.jq];
      moduleScript = hmConfig.home.activation.retrieveOpnixSecrets;
    } ''
      config_file=$(printf '%s\n' "$moduleScript" | grep -o '/nix/store/[^ ]*-hm-opnix-declarative-secrets.json' | head -n1)
      jq -e --arg marker '<OPNIX:${builtins.hashString "sha256" "defaultSecret"}>' '
        any(.secrets[]; .name == "defaultSecret" and .kind == "field" and .reference == "op://Example/Service/password") and
        any(.secrets[]; .name == "fileSecret" and .kind == "file") and
        any(.templates[]; .name == "appConfig" and .path == "/home/opnix-test/.config/app.conf" and .content == ("password=" + $marker) and .owner == "opnix-test" and .group == "${
        if pkgs.stdenv.isDarwin
        then "staff"
        else "users"
      }" and .mode == "0600")
      ' "$config_file" >/dev/null
      touch $out
    '';

  darwin-module-evaluation = assert !(darwinConfig.services.onepassword-secrets.placeholder ? fileSecret);
  assert darwinConfig.services.onepassword-secrets.placeholder.defaultSecret == "<OPNIX:7ab73f276766e0ae10ed5dbd2bad50a3db7dda14a35a05a91df135dd3769adfa>";
  assert darwinConfig.services.onepassword-secrets.templatePaths.appConfig == "/usr/local/var/opnix/secrets/app.conf";
  assert darwinConfig.services.onepassword-secrets.secrets.defaultSecret.kind == "field";
  assert darwinConfig.services.onepassword-secrets.secrets.fileSecret.kind == "file";
  assert darwinConfig.launchd.daemons.opnix-secrets.script != "";
  assert !(darwinConfig.launchd.daemons.opnix-secrets.serviceConfig ? ProgramArguments);
  assert darwinConfig.launchd.daemons.opnix-secrets.serviceConfig.Label == "org.nixos.opnix-secrets";
  assert darwinConfig.launchd.daemons.opnix-secrets.serviceConfig.RunAtLoad;
  assert darwinConfig.launchd.daemons.opnix-secrets.serviceConfig.KeepAlive.SuccessfulExit == false;
  assert darwinConfig.launchd.daemons.opnix-secrets.serviceConfig.StandardErrorPath == "/var/log/opnix-secrets.log";
  assert darwinConfig.launchd.daemons.opnix-secrets.serviceConfig.StandardOutPath == "/var/log/opnix-secrets.log";
    pkgs.runCommand "opnix-darwin-module-evaluation" {
      nativeBuildInputs = [pkgs.jq];
      moduleScript = darwinConfig.launchd.daemons.opnix-secrets.script;
    } ''
      config_file=$(printf '%s\n' "$moduleScript" | grep -o '/nix/store/[^ ]*-opnix-declarative-secrets.json' | head -n1)
      jq -e --arg marker '<OPNIX:${builtins.hashString "sha256" "defaultSecret"}>' '
        any(.secrets[]; .name == "defaultSecret" and .kind == "field" and .reference == "op://Example/Service/password") and
        any(.secrets[]; .name == "fileSecret" and .kind == "file") and
        any(.templates[]; .name == "appConfig" and .path == "/usr/local/var/opnix/secrets/app.conf" and .content == ("password=" + $marker) and .owner == "root" and .group == "root" and .mode == "0600" and .services == ["com.example.app"])
      ' "$config_file" >/dev/null
      touch $out
    '';
})
// pkgs.lib.optionalAttrs pkgs.stdenv.isLinux (let
  nixosConfig = polling:
    (nixpkgs.lib.nixosSystem {
      inherit system;
      modules = [
        ./module.nix
        ({config, ...}: {
          system.stateVersion = "26.05";
          services.onepassword-secrets = {
            enable = true;
            secrets.testSecret.reference = "op://Example/Service/password";
            secrets.testFile = {
              reference = "op://Example/Document/archive.bin";
              kind = "file";
            };
            templates.appConfig = {
              path = "app.conf";
              content = "password=${config.services.onepassword-secrets.placeholder.testSecret}";
              services.example = {};
            };
            systemdIntegration.polling = polling;
          };
        })
      ];
    }).config;

  defaultPollingConfig = nixosConfig {};
  enabledPollingConfig = nixosConfig {
    enable = true;
    interval = "45min";
  };
in {
  module-evaluation = assert !(defaultPollingConfig.services.onepassword-secrets.placeholder ? testFile);
  assert defaultPollingConfig.services.onepassword-secrets.placeholder.testSecret == "<OPNIX:36f0f4b6056ac4d0a4b7dbb7c94597c1d44c9d6c84dc370121deedbde5049560>";
  assert defaultPollingConfig.services.onepassword-secrets.templatePaths.appConfig == "/var/lib/opnix/secrets/app.conf";
  assert builtins.elem "opnix-secrets.service" defaultPollingConfig.systemd.services.example.after;
  assert builtins.elem "opnix-secrets.service" defaultPollingConfig.systemd.services.example.wants;
  assert !(defaultPollingConfig.systemd.services ? opnix-secrets-poll);
  assert !(defaultPollingConfig.systemd.timers ? opnix-secrets-poll);
  assert defaultPollingConfig.services.onepassword-secrets.secrets.testSecret.kind == "field";
  assert defaultPollingConfig.services.onepassword-secrets.secrets.testFile.kind == "file";
  assert enabledPollingConfig.systemd.services.opnix-secrets-poll.serviceConfig.Type == "oneshot";
  assert !(enabledPollingConfig.systemd.services.opnix-secrets-poll.serviceConfig ? RemainAfterExit);
  assert nixpkgs.lib.hasInfix "systemctl stop opnix-secrets-watcher.path" enabledPollingConfig.systemd.services.opnix-secrets-poll.script;
  assert nixpkgs.lib.hasInfix "trap restore_watcher EXIT" enabledPollingConfig.systemd.services.opnix-secrets-poll.script;
  assert enabledPollingConfig.systemd.timers.opnix-secrets-poll.timerConfig.OnActiveSec == "45min";
  assert enabledPollingConfig.systemd.timers.opnix-secrets-poll.timerConfig.OnUnitActiveSec == "45min";
  assert enabledPollingConfig.systemd.timers.opnix-secrets-poll.timerConfig.Unit == "opnix-secrets-poll.service";
    pkgs.runCommand "opnix-module-evaluation" {
      nativeBuildInputs = [pkgs.jq];
      moduleScript = defaultPollingConfig.systemd.services.opnix-secrets.script;
    } ''
      config_file=$(printf '%s\n' "$moduleScript" | grep -o '/nix/store/[^ ]*-opnix-declarative-secrets.json' | head -n1)
      jq -e --arg marker '<OPNIX:${builtins.hashString "sha256" "testSecret"}>' '
        any(.secrets[]; .name == "testSecret" and .kind == "field" and .reference == "op://Example/Service/password") and
        any(.secrets[]; .name == "testFile" and .kind == "file") and
        any(.templates[]; .name == "appConfig" and .path == "/var/lib/opnix/secrets/app.conf" and .content == ("password=" + $marker) and .owner == "root" and .group == "root" and .mode == "0600" and .services.example.restart == true)
      ' "$config_file" >/dev/null
      touch $out
    '';
})
