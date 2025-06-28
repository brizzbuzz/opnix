{
  config,
  lib,
  pkgs,
  ...
}: let
  cfg = config.services.onepassword-secrets;

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
      default = "/var/lib/opnix/secrets";
      description = "Directory to store retrieved secrets";
    };

    # New option for users that should have access to the token
    users = lib.mkOption {
      type = lib.types.listOf lib.types.str;
      default = [];
      description = "Users that should have access to the 1Password token through group membership";
      example = ["alice" "bob"];
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
            description = "Custom path for the secret file. If null, uses pathTemplate or outputDir + secret name";
            example = "/etc/ssl/certs/app.pem";
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
        };
      });
      default = {};
      description = ''
        Declarative secrets configuration (GitHub #11).
        Keys are secret names, values are secret configurations.
      '';
      example = {
        "database/password" = {
          reference = "op://Vault/Database/password";
        };
        "ssl/cert" = {
          reference = "op://Vault/SSL/certificate";
          path = "/etc/ssl/certs/app.pem";
          owner = "caddy";
          group = "caddy";
          mode = "0644";
          symlinks = ["/etc/ssl/certs/legacy.pem"];
        };
        "service/config" = {
          reference = "op://Vault/Service/config";
          variables = {
            service = "postgresql";
            environment = "prod";
          };
        };
      };
    };

    pathTemplate = lib.mkOption {
      type = lib.types.nullOr lib.types.str;
      default = null;
      description = ''
        Path template for secrets when no explicit path is specified.
        Variables can be substituted using {variable} syntax.
      '';
      example = "/etc/secrets/{service}/{name}";
    };

    defaults = lib.mkOption {
      type = lib.types.attrsOf lib.types.str;
      default = {};
      description = "Default variables for path template substitution";
      example = {
        environment = "production";
        service = "default";
      };
    };
  };

  config = lib.mkIf cfg.enable (let
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
            cfg.secrets;
          pathTemplate = cfg.pathTemplate;
          defaults = cfg.defaults;
        })
      else null;

    # Collect all config files
    allConfigFiles = lib.filter (f: f != null) (
      cfg.configFiles
      ++ (lib.optional hasDeclarativeSecrets declarativeConfigFile)
    );
  in
    lib.mkMerge [
      # Validation assertions
      {
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
      }

      # Main configuration
      {
        # Create the opnix group
        users.groups.${opnixGroup} = {};

        # Add specified users to the opnix group
        users.users = lib.mkMerge (map (user: {
            ${user}.extraGroups = [opnixGroup];
          })
          cfg.users);

        # Create systemd service instead of activation script
        systemd.services.opnix-secrets = {
          description = "OpNix Secret Management";
          wantedBy = ["multi-user.target"];
          after = ["network.target"];
          wants = ["network.target"];

          serviceConfig = {
            Type = "oneshot";
            RemainAfterExit = true;
            Restart = "on-failure";
            RestartSec = 30;
            User = "root";
            Group = opnixGroup;
          };

          script = ''
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
          '';
        };

        # Add declarative path references for secrets (GitHub #11)
        services.onepassword-secrets.secretPaths =
          lib.mapAttrs (
            name: secret:
              if secret.path != null
              then secret.path
              else "${cfg.outputDir}/${name}"
          )
          cfg.secrets;
      }
    ]);
}
