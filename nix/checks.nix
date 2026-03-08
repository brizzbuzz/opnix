{
  home-manager,
  pkgs,
  src,
}: let
  compatModule = {lib, ...}: {
    options = {
      assertions = lib.mkOption {
        type = lib.types.listOf lib.types.anything;
        default = [];
      };

      environment.systemPackages = lib.mkOption {
        type = lib.types.listOf lib.types.anything;
        default = [];
      };

      launchd.daemons = lib.mkOption {
        type = lib.types.attrsOf lib.types.anything;
        default = {};
      };

      system.activationScripts = lib.mkOption {
        type = lib.types.attrsOf lib.types.anything;
        default = {};
      };

      systemd.paths = lib.mkOption {
        type = lib.types.attrsOf lib.types.anything;
        default = {};
      };

      systemd.services = lib.mkOption {
        type = lib.types.attrsOf lib.types.anything;
        default = {};
      };

      users.groups = lib.mkOption {
        type = lib.types.attrsOf lib.types.anything;
        default = {};
      };

      users.knownGroups = lib.mkOption {
        type = lib.types.listOf lib.types.str;
        default = [];
      };

      users.users = lib.mkOption {
        type = lib.types.attrsOf lib.types.anything;
        default = {};
      };
    };
  };

  extractDeclarativeConfigPath = script: let
    normalizedScript = pkgs.lib.replaceStrings ["\n" "\t"] [" " " "] script;
    matches = builtins.match ''.*-config ([^[:space:]]*opnix-declarative-secrets\.json).*'' normalizedScript;
  in
    if matches == null
    then throw "Could not extract declarative config path from rendered script"
    else builtins.elemAt matches 0;

  hmEval = home-manager.lib.homeManagerConfiguration {
    inherit pkgs;
    modules = [
      ../nix/hm-module.nix
      {
        home.username = "alice";
        home.homeDirectory = "/home/alice";
        home.stateVersion = "24.11";

        programs.onepassword-secrets = {
          enable = true;
          secrets = {
            relSecret = {
              reference = "op://Vault/Item/field";
              path = ".ssh/id_rsa";
            };

            absSecret = {
              reference = "op://Vault/Item/field";
              path = "/run/secrets/user/mySecret";
            };

            defaultSecret = {
              reference = "op://Vault/Item/field";
            };
          };
        };
      }
    ];
  };

  hmSecretPaths = hmEval.config.programs.onepassword-secrets.secretPaths;
  hmActivation = hmEval.config.home.activation.createOpnixDirs.data;

  hmInvalidKeyEval = builtins.tryEval (
    (home-manager.lib.homeManagerConfiguration {
      inherit pkgs;
      modules = [
        ../nix/hm-module.nix
        {
          home.username = "alice";
          home.homeDirectory = "/home/alice";
          home.stateVersion = "24.11";

          programs.onepassword-secrets = {
            enable = true;
            secrets."bad-key".reference = "op://Vault/Item/field";
          };
        }
      ];
    }).config.programs.onepassword-secrets.secretPaths
  );

  nixosEval = pkgs.lib.evalModules {
    specialArgs = {inherit pkgs;};
    modules = [
      compatModule
      ../nix/module.nix
      {
        services.onepassword-secrets = {
          enable = true;
          outputDir = "/run/opnix";
          users = ["alice"];
          systemdIntegration = {
            enable = true;
            services = ["caddy"];
            changeDetection.hashFile = "/var/lib/opnix/test-hashes.json";
          };
          secrets = {
            absSecret = {
              reference = "op://Vault/Item/field";
              path = "/etc/ssl/private/app.key";
            };
            defaultSecret.reference = "op://Vault/Item/default";
            serviceToken = {
              reference = "op://Vault/Item/token";
              services = ["nginx"];
            };
          };
        };
      }
    ];
  };

  nixosSecretPaths = nixosEval.config.services.onepassword-secrets.secretPaths;
  nixosScript = nixosEval.config.systemd.services.opnix-secrets.script;

  nixosInvalidKeyEval = builtins.tryEval (
    (pkgs.lib.evalModules {
      specialArgs = {inherit pkgs;};
      modules = [
        compatModule
        ../nix/module.nix
        {
          services.onepassword-secrets = {
            enable = true;
            secrets."bad-key".reference = "op://Vault/Item/field";
          };
        }
      ];
    }).config.services.onepassword-secrets.secretPaths
  );

  nixosInvalidModeEval =
    (pkgs.lib.evalModules {
      specialArgs = {inherit pkgs;};
      modules = [
        compatModule
        ../nix/module.nix
        {
          services.onepassword-secrets = {
            enable = true;
            secrets.badMode = {
              reference = "op://Vault/Item/field";
              mode = "invalid";
            };
          };
        }
      ];
    }).config.assertions;

  nixosMissingConfigEval =
    (pkgs.lib.evalModules {
      specialArgs = {inherit pkgs;};
      modules = [
        compatModule
        ../nix/module.nix
        {
          services.onepassword-secrets.enable = true;
        }
      ];
    }).config.assertions;

  darwinEval = pkgs.lib.evalModules {
    specialArgs = {inherit pkgs;};
    modules = [
      compatModule
      ../nix/darwin-module.nix
      {
        services.onepassword-secrets = {
          enable = true;
          groupId = 777;
          users = ["alice"];
          outputDir = "/usr/local/var/opnix/test-secrets";
          secrets = {
            absSecret = {
              reference = "op://Vault/Item/field";
              path = "/etc/ssl/certs/app.pem";
            };
            defaultSecret.reference = "op://Vault/Item/default";
          };
        };
      }
    ];
  };

  darwinSecretPaths = darwinEval.config.services.onepassword-secrets.secretPaths;
  darwinProgramArguments = darwinEval.config.launchd.daemons.opnix-secrets.serviceConfig.ProgramArguments;
  darwinScript = builtins.elemAt darwinProgramArguments 2;

  darwinInvalidModeEval =
    (pkgs.lib.evalModules {
      specialArgs = {inherit pkgs;};
      modules = [
        compatModule
        ../nix/darwin-module.nix
        {
          services.onepassword-secrets = {
            enable = true;
            secrets.badMode = {
              reference = "op://Vault/Item/field";
              mode = "invalid";
            };
          };
        }
      ];
    }).config.assertions;
  desiredCoverageThreshold = 80.0;
in {
  hm-module-eval = assert hmSecretPaths.relSecret == "/home/alice/.ssh/id_rsa";
  assert hmSecretPaths.absSecret == "/run/secrets/user/mySecret";
  assert hmSecretPaths.defaultSecret == "/home/alice/defaultSecret";
  assert pkgs.lib.hasInfix "mkdir -p /run/secrets/user" hmActivation;
  assert hmInvalidKeyEval.success == false;
    pkgs.runCommand "opnix-hm-module-eval" {} "touch $out";

  nixos-module-eval = assert nixosSecretPaths.absSecret == "/etc/ssl/private/app.key";
  assert nixosSecretPaths.defaultSecret == "/run/opnix/defaultSecret";
  assert nixosSecretPaths.serviceToken == "/run/opnix/serviceToken";
  assert nixosEval.config.users.users.alice.extraGroups == ["onepassword-secrets"];
  assert nixosEval.config.systemd.services.nginx.after == ["opnix-secrets.service"];
  assert nixosEval.config.systemd.services.caddy.wants == ["opnix-secrets.service"];
  assert nixosEval.config.systemd.paths.opnix-secrets-watcher.pathConfig.PathModified == "/run/opnix";
  assert pkgs.lib.hasInfix "mkdir -p /run/opnix" nixosScript;
  assert pkgs.lib.hasInfix "-token-file /etc/opnix-token" nixosScript;
  assert pkgs.lib.hasInfix "-output /run/opnix" nixosScript;
  assert nixosInvalidKeyEval.success == false;
  assert builtins.any (assertion: !assertion.assertion && pkgs.lib.hasInfix "valid octal permission" assertion.message) nixosInvalidModeEval;
  assert builtins.any (assertion: !assertion.assertion && pkgs.lib.hasInfix "At least one of configFiles or secrets must be specified" assertion.message) nixosMissingConfigEval;
    pkgs.runCommand "opnix-nixos-module-eval"
    {
      nativeBuildInputs = [pkgs.jq];
      declarativeConfig = extractDeclarativeConfigPath nixosScript;
      renderedScript = nixosScript;
    } ''
      printf '%s' "$renderedScript" | grep -F -- "mkdir -p /run/opnix"
      printf '%s' "$renderedScript" | grep -F -- "-config $declarativeConfig"
      jq -e '.secrets | length == 3' "$declarativeConfig" > /dev/null
      jq -e '.secrets[0].path == "/etc/ssl/private/app.key"' "$declarativeConfig" > /dev/null
      jq -e '.secrets[1].path == "defaultSecret"' "$declarativeConfig" > /dev/null
      jq -e '.secrets[2].services == ["nginx"]' "$declarativeConfig" > /dev/null
      jq -e '.systemdIntegration.enable == true' "$declarativeConfig" > /dev/null
      jq -e '.systemdIntegration.changeDetection.hashFile == "/var/lib/opnix/test-hashes.json"' "$declarativeConfig" > /dev/null
      touch $out
    '';

  darwin-module-eval = assert darwinSecretPaths.absSecret == "/etc/ssl/certs/app.pem";
  assert darwinSecretPaths.defaultSecret == "/usr/local/var/opnix/test-secrets/defaultSecret";
  assert darwinEval.config.users.knownGroups == ["onepassword-secrets"];
  assert darwinEval.config.users.groups.onepassword-secrets.gid == 777;
  assert darwinEval.config.users.users.alice.packages != [];
  assert darwinEval.config.launchd.daemons.opnix-secrets.serviceConfig.RunAtLoad == true;
  assert builtins.length darwinProgramArguments == 3;
  assert pkgs.lib.hasInfix "mkdir -p /usr/local/var/opnix/test-secrets" darwinScript;
  assert pkgs.lib.hasInfix "-config" darwinScript;
  assert pkgs.lib.hasInfix "-output /usr/local/var/opnix/test-secrets" darwinScript;
  assert darwinEval.config.system.activationScripts.extraActivation.text != "";
  assert builtins.any (assertion: !assertion.assertion && pkgs.lib.hasInfix "valid octal permission" assertion.message) darwinInvalidModeEval;
    pkgs.runCommand "opnix-darwin-module-eval"
    {
      nativeBuildInputs = [pkgs.jq];
      declarativeConfig = extractDeclarativeConfigPath darwinScript;
      renderedScript = darwinScript;
    } ''
      printf '%s' "$renderedScript" | grep -F -- "mkdir -p /usr/local/var/opnix/test-secrets"
      printf '%s' "$renderedScript" | grep -F -- "-config $declarativeConfig"
      jq -e '.secrets | length == 2' "$declarativeConfig" > /dev/null
      jq -e '.secrets[0].path == "/etc/ssl/certs/app.pem"' "$declarativeConfig" > /dev/null
      jq -e '.secrets[1].path == "defaultSecret"' "$declarativeConfig" > /dev/null
      touch $out
    '';

  # Run tests
  go-tests = pkgs.stdenv.mkDerivation {
    name = "opnix-go-tests";
    inherit src;

    nativeBuildInputs = [pkgs.go];

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

  go-coverage = pkgs.stdenv.mkDerivation {
    name = "opnix-go-coverage";
    inherit src;

    nativeBuildInputs = [pkgs.gawk pkgs.go];

    buildPhase = ''
      export GOPATH=$TMPDIR/go
      export GOCACHE=$TMPDIR/go-cache
      export GO111MODULE=on

      mkdir -p $TMPDIR/workspace
      cd $TMPDIR/workspace

      cp -r $src/* .

      go mod download
      go test ./... -covermode=atomic -coverprofile=coverage.out

      total=$(go tool cover -func=coverage.out | awk '/^total:/ { gsub(/%/, "", $3); print $3 }')
      if [ -z "$total" ]; then
        echo "Failed to determine total Go coverage" >&2
        exit 1
      fi

      echo "Total Go coverage: $total%"
      awk "BEGIN { exit !($total >= ${toString desiredCoverageThreshold}) }"
    '';

    installPhase = ''
      mkdir -p $out
      cp coverage.out $out/coverage.out
      go tool cover -func=coverage.out > $out/summary.txt
    '';
  };

  # Run golangci-lint
  go-lint = pkgs.stdenv.mkDerivation {
    name = "opnix-go-lint";
    inherit src;

    nativeBuildInputs = [pkgs.go pkgs.golangci-lint];

    buildPhase = ''
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
        in "echo -n '${cfg}' >> .golangci.yaml"
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
