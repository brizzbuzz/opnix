{
  config,
  lib,
  pkgs,
  ...
}: let
  cfg = config.programs.onepassword-secrets;

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

  # Create a new pkgs instance with our overlay
  pkgsWithOverlay = import pkgs.path {
    inherit (pkgs) system;
    overlays = [
      (final: prev: {
        opnix = import ./package.nix {pkgs = final;};
      })
    ];
  };

  # Format type for secrets with home directory expansion
  secretType = lib.types.submodule {
    options = {
      path = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        description = ''
          Path where the secret will be stored, relative to home directory.
          If null, uses the secret name. For example: ".config/Yubico/u2f_keys" or ".ssh/id_rsa"
        '';
        example = ".config/Yubico/u2f_keys";
      };

      reference = lib.mkOption {
        type = lib.types.str;
        description = "1Password reference in the format op://vault/item/field";
        example = "op://Personal/ssh-key/private-key";
      };

      owner = lib.mkOption {
        type = lib.types.str;
        default = config.home.username;
        description = "User who owns the secret file (defaults to home user)";
      };

      group = lib.mkOption {
        type = lib.types.str;
        default = "users";
        description = "Group that owns the secret file";
      };

      mode = lib.mkOption {
        type = lib.types.str;
        default = "0600";
        description = "File permissions in octal notation";
        example = "0644";
      };
    };
  };
in {
  options.programs.onepassword-secrets = {
    enable = lib.mkEnableOption "1Password secrets integration";

    configFiles = lib.mkOption {
      type = lib.types.listOf lib.types.path;
      default = [];
      description = "List of secrets configuration files (GitHub #3)";
      example = [./personal-secrets.json ./work-secrets.json];
    };

    tokenFile = lib.mkOption {
      type = lib.types.path;
      default = "/etc/opnix-token";
      description = ''
        Path to file containing the 1Password service account token.
        The file should contain only the token and should have appropriate permissions (640).

        You can set up the token using the opnix CLI:
          opnix token set
          # or with a custom path:
          opnix token set -path /path/to/token
      '';
    };

    secrets = lib.mkOption {
      type = lib.types.attrsOf secretType;
      default = {};
      description = ''
        Declarative secrets configuration (GitHub #11).
        Keys are secret names, values are secret configurations.
        Paths are relative to home directory.
      '';
      example = {
        sshPrivateKey = {
          reference = "op://Personal/SSH/private-key";
          path = ".ssh/id_rsa";
          mode = "0600";
        };
        configApiKey = {
          reference = "op://Work/API/key";
          path = ".config/myapp/api-key";
          mode = "0640";
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
    # Always define secretPaths to prevent evaluation errors (fixes GitHub issue)
    {
      programs.onepassword-secrets.secretPaths =
        if cfg.enable && cfg.secrets != {}
        then
          lib.mapAttrs (
            name: secret: let
              secretPath =
                if secret.path != null
                then secret.path
                else name;
            in "${config.home.homeDirectory}/${secretPath}"
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
          pkgs.writeText "hm-opnix-declarative-secrets.json" (builtins.toJSON {
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
            message = "OpNix Home Manager: At least one of configFiles or secrets must be specified";
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
      home.packages = [pkgsWithOverlay.opnix];

      # Create necessary directories for declarative secrets
      home.activation.createOpnixDirs = lib.hm.dag.entryBefore ["checkLinkTargets"] ''
        # Create parent directories for all declarative secrets
        ${lib.concatMapStringsSep "\n" (name: let
          secret = cfg.secrets.${name};
          secretPath =
            if secret.path != null
            then secret.path
            else name;
        in ''
          $DRY_RUN_CMD mkdir -p "''${HOME}/${lib.escapeShellArg (builtins.dirOf secretPath)}"
        '') (builtins.attrNames cfg.secrets)}
      '';

      # Retrieve secrets during activation
      home.activation.retrieveOpnixSecrets = lib.hm.dag.entryAfter ["createOpnixDirs"] ''
        # Handle missing token file gracefully
        if [ ! -f ${lib.escapeShellArg cfg.tokenFile} ]; then
          echo "WARNING: Token file ${cfg.tokenFile} does not exist!" >&2
          echo "INFO: Using existing secrets, skipping updates" >&2
          echo "INFO: Run 'opnix token set' to configure the token" >&2
          exit 0
        fi

        if [ ! -r ${lib.escapeShellArg cfg.tokenFile} ]; then
          echo "ERROR: Cannot read system token at ${cfg.tokenFile}" >&2
          echo "INFO: Make sure the system token can be accessed by your user" >&2
          exit 1
        fi

        # Retrieve secrets for each config file
        ${lib.concatMapStringsSep "\n" (configFile: ''
            echo "Processing config file: ${configFile}"
            $DRY_RUN_CMD ${pkgsWithOverlay.opnix}/bin/opnix secret \
              -token-file ${lib.escapeShellArg cfg.tokenFile} \
              -config ${configFile} \
              -output "$HOME"
          '')
          allConfigFiles}
      '';
    }))
  ];
}
