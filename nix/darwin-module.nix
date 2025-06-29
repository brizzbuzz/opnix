{
  config,
  lib,
  pkgs,
  ...
}: let
  cfg = config.services.onepassword-secrets;

  # Validate that secret keys use proper Nix variable naming (camelCase)
  # Valid: databasePassword, sslCert, myApiKey
  # Invalid: "database/password", "ssl-cert", "my_api_key"
  isValidNixVariableName = key:
    builtins.match "^[a-z][a-zA-Z0-9]*$" key != null;

  # Validate all secret keys
  validateSecretKeys = secrets: let
    invalidKeys = lib.filter (key: !isValidNixVariableName key) (lib.attrNames secrets);
  in
    if invalidKeys != []
    then throw "Invalid secret key names. OpNix requires camelCase variable names like 'databasePassword', not path-like strings. Invalid keys: ${lib.concatStringsSep ", " invalidKeys}"
    else secrets;

  # Utility function to convert camelCase to kebab-case for file paths
  # Examples: databasePassword -> database-password, sslCert -> ssl

  # Create a new pkgs instance with our overlay
  pkgsWithOverlay = import pkgs.path {
    inherit (pkgs) system;
    overlays = [
      (final: prev: {
        opnix = import ./package.nix {pkgs = final;};
      })
    ];
  };

  # Create a system group for opnix token access
  opnixGroup = "onepassword-secrets";
in {
  options.services.onepassword-secrets = {
    enable = lib.mkEnableOption "1Password secrets integration";

    tokenFile = lib.mkOption {
      type = lib.types.path;
      default = "/etc/opnix-token";
      description = ''
        Path to file containing the 1Password service account token.
        The file should contain only the token and should have appropriate permissions (640).
        Will be readable by members of the ${opnixGroup} group.

        You can set up the token using the opnix CLI:
          opnix token set
          # or with a custom path:
          opnix token set -path /path/to/token
      '';
    };

    configFiles = lib.mkOption {
      type = lib.types.listOf lib.types.path;
      default = [];
      description = "List of secrets configuration files (GitHub #3)";
      example = [./database-secrets.json ./api-secrets.json];
    };

    outputDir = lib.mkOption {
      type = lib.types.str;
      default = "/usr/local/var/opnix/secrets";
      description = "Directory to store retrieved secrets";
    };

    # New option for users that should have access to the token
    users = lib.mkOption {
      type = lib.types.listOf lib.types.str;
      default = [];
      description = "Users that should have access to the 1Password token through group membership";
      example = ["alice" "bob"];
    };

    # nix-darwin will not assign a default gid
    groupId = lib.mkOption {
      type = lib.types.ints.between 500 1000;
      default = 600;
      description = "An unused group id to assign to the Opnix group. You can see existing groups by running `dscl . list /Groups PrimaryGroupID | tr -s ' ' | sort -n -t ' ' -k2,2`.";
      example = 555;
    };

    secrets = lib.mkOption {
      type = lib.types.attrsOf (lib.types.submodule {
        options = {
          reference = lib.mkOption {
            type = lib.types.str;
            description = "1Password reference in the format op://Vault/Item/field";
            example = "op://Homelab/Database/password";
          };

          path = lib.mkOption {
            type = lib.types.nullOr lib.types.str;
            default = null;
            description = "Custom path for the secret file. If null, uses outputDir + secret name";
            example = "/etc/ssl/certs/app.pem";
          };

          owner = lib.mkOption {
            type = lib.types.str;
            default = "root";
            description = "User who owns the secret file";
            example = "caddy";
          };

          group = lib.mkOption {
            type = lib.types.str;
            default = "root";
            description = "Group that owns the secret file";
            example = "caddy";
          };

          mode = lib.mkOption {
            type = lib.types.str;
            default = "0600";
            description = "File permissions in octal notation";
            example = "0644";
          };

          symlinks = lib.mkOption {
            type = lib.types.listOf lib.types.str;
            default = [];
            description = "List of symlink paths that should point to this secret";
            example = ["/etc/ssl/certs/legacy.pem" "/opt/service/ssl/cert.pem"];
          };

          variables = lib.mkOption {
            type = lib.types.attrsOf lib.types.str;
            default = {};
            description = "Variables for path template substitution";
            example = {
              service = "postgresql";
              environment = "prod";
            };
          };

          services = lib.mkOption {
            type = lib.types.listOf lib.types.str;
            default = [];
            description = "List of services to restart when this secret changes (macOS services)";
            example = ["com.example.myservice"];
          };
        };
      });
      default = {};
      description = ''
        Declarative secrets configuration (GitHub #11).
        Keys are secret names, values are secret configurations.
      '';
      example = {
        databasePassword = {
          reference = "op://Vault/Database/password";
        };
        sslCert = {
          reference = "op://Vault/SSL/certificate";
          path = "/etc/ssl/certs/app.pem";
          owner = "caddy";
          group = "caddy";
          mode = "0644";
        };
      };
    };

    secretPaths = lib.mkOption {
      type = lib.types.attrsOf lib.types.str;
      default = {};
      description = ''
        Computed paths for declarative secrets (GitHub #11).
        This is automatically populated and provides declarative references
        to secret file paths for use in other configuration sections.
      '';
    };
  };

  config = lib.mkMerge [
    # Always define secretPaths to prevent evaluation errors (fixes MMI-87)
    {
      services.onepassword-secrets.secretPaths =
        if cfg.enable && cfg.secrets != {}
        then
          lib.mapAttrs (
            name: secret:
              if secret.path != null
              then secret.path
              else "${cfg.outputDir}/${name}"
          )
          (validateSecretKeys cfg.secrets)
        else {};
    }

    # Main configuration only when enabled
    (lib.mkIf cfg.enable (let
      # Validate configuration
      hasMultipleConfigs = cfg.configFiles != [];
      hasDeclarativeSecrets = cfg.secrets != {};

      # At least one configuration method must be specified
      configCount = lib.length (lib.filter (x: x) [hasMultipleConfigs hasDeclarativeSecrets]);

      # Generate a temporary config file from declarative secrets
      declarativeConfigFile =
        if hasDeclarativeSecrets
        then
          pkgs.writeText "opnix-declarative-secrets.json" (builtins.toJSON {
            secrets =
              lib.mapAttrsToList (name: secret: {
                path =
                  if secret.path != null
                  then secret.path
                  else name;
                reference = secret.reference;
                owner = secret.owner;
                group = secret.group;
                mode = secret.mode;
                symlinks = secret.symlinks;
                variables = secret.variables;
              })
              (validateSecretKeys cfg.secrets);
          })
        else null;

      # Collect all config files
      allConfigFiles = lib.filter (f: f != null) (
        cfg.configFiles
        ++ (lib.optional hasDeclarativeSecrets declarativeConfigFile)
      );
    in {
      # Validation assertions
      assertions =
        [
          {
            assertion = configCount > 0;
            message = "OpNix: At least one of configFiles or secrets must be specified";
          }
        ]
        ++ (lib.flatten (lib.mapAttrsToList (name: secret: [
            {
              assertion = builtins.match "^[0-7]{3,4}$" secret.mode != null;
              message = "OpNix secret '${name}': mode '${secret.mode}' is not a valid octal permission (e.g., 0644, 0600)";
            }
          ])
          cfg.secrets));

      # Main configuration
      # Let nix-darwin know it's allowed to mess with this group
      users.knownGroups = [opnixGroup];

      # Create the opnix group
      users.groups.${opnixGroup} = {
        members = cfg.users;
        gid = cfg.groupId;
      };

      # Add the opnix binary to the users environment
      users.users = builtins.listToAttrs (map
        (username: {
          name = username;
          value = {
            packages = [
              pkgsWithOverlay.opnix
            ];
          };
        })
        cfg.users);

      # Create launchd service for macOS
      launchd.daemons.opnix-secrets = {
        serviceConfig = {
          Label = "org.nixos.opnix-secrets";
          ProgramArguments = [
            "/bin/sh"
            "-c"
            ''
              # Ensure output directory exists with correct permissions
              mkdir -p ${cfg.outputDir}
              chmod 750 ${cfg.outputDir}

              # Set up token file with correct group permissions if it exists
              if [ -f ${cfg.tokenFile} ]; then
                # Ensure token file has correct ownership and permissions
                chown root:${opnixGroup} ${cfg.tokenFile}
                chmod 640 ${cfg.tokenFile}
              fi

              # Handle missing token file gracefully - don't fail system boot
              if [ ! -f ${cfg.tokenFile} ]; then
                echo "WARNING: Token file ${cfg.tokenFile} does not exist!" >&2
                echo "INFO: Using existing secrets, skipping updates" >&2
                echo "INFO: Run 'opnix token set' to configure the token" >&2
                exit 0
              fi

              # Validate token file permissions
              if [ ! -r ${cfg.tokenFile} ]; then
                echo "ERROR: Token file ${cfg.tokenFile} is not readable!" >&2
                echo "INFO: Check file permissions or group membership" >&2
                exit 1
              fi

              # Validate token is not empty
              if [ ! -s ${cfg.tokenFile} ]; then
                echo "ERROR: Token file is empty!" >&2
                echo "INFO: Run 'opnix token set' to configure the token" >&2
                exit 1
              fi

              # Run the secrets retrieval tool for each config file
              ${lib.concatMapStringsSep "\n" (configFile: ''
                  echo "Processing config file: ${configFile}"
                  ${pkgsWithOverlay.opnix}/bin/opnix secret \
                    -token-file ${cfg.tokenFile} \
                    -config ${configFile} \
                    -output ${cfg.outputDir}
                '')
                allConfigFiles}
            ''
          ];
          RunAtLoad = true;
          KeepAlive = {
            SuccessfulExit = false;
          };
          StandardErrorPath = "/var/log/opnix-secrets.log";
          StandardOutPath = "/var/log/opnix-secrets.log";
        };
      };

      # nix-darwin doesn't support arbitrary activation script names,
      # so have to use a specific one.
      #
      # See source for details: https://github.com/nix-darwin/nix-darwin/blob/2f140d6ac8840c6089163fb43ba95220c230f22b/modules/system/activation-scripts.nix#L118
      system.activationScripts.extraActivation.text = ''
        # OpNix secrets are now managed by launchd service instead of activation script
        echo "INFO: OpNix secrets managed by launchd service"
      '';
    }))
  ];
}
