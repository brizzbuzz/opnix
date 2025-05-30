{ config, lib, pkgs, ... }:

let
  cfg = config.programs.onepassword-secrets;

  # Create a new pkgs instance with our overlay
  pkgsWithOverlay = import pkgs.path {
    inherit (pkgs) system;
    overlays = [
      (final: prev: {
        opnix = import ./package.nix { pkgs = final; };
      })
    ];
  };

  # Format type for secrets with home directory expansion
  secretType = lib.types.submodule {
    options = {
      path = lib.mkOption {
        type = lib.types.str;
        description = ''
          Path where the secret will be stored, relative to home directory.
          For example: ".config/Yubico/u2f_keys" or ".ssh/id_rsa"
        '';
        example = ".config/Yubico/u2f_keys";
      };

      reference = lib.mkOption {
        type = lib.types.str;
        description = "1Password reference in the format op://vault/item/field";
        example = "op://Personal/ssh-key/private-key";
      };
    };
  };
in {
  options.programs.onepassword-secrets = {
    enable = lib.mkEnableOption "1Password secrets integration";

    configFile = lib.mkOption {
      type = lib.types.path;
      default = "${config.xdg.configHome}/opnix/secrets.json";
      description = "Path to secrets configuration file";
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
      type = lib.types.listOf secretType;
      default = [];
      description = ''
        List of secrets to manage. Each secret's path is relative to the home directory.
        For example, to store a secret at ~/.config/myapp/secret, use path = ".config/myapp/secret"
      '';
      example = lib.literalExpression ''
        [
          {
            path = ".config/Yubico/u2f_keys";
            reference = "op://vault/u2f/keys";
          }
          {
            path = ".ssh/id_rsa";
            reference = "op://vault/ssh/key";
          }
        ]
      '';
    };
  };

  config = lib.mkIf cfg.enable {
    assertions = [
      {
        assertion = cfg.secrets != [];
        message = "No secrets configured for onepassword-secrets. Did you forget to add secrets?";
      }
    ];

    home.packages = [ pkgsWithOverlay.opnix ];

    # Create necessary directories
    home.activation.createOpnixDirs = lib.hm.dag.entryBefore ["checkLinkTargets"] ''
      # Create config directory
      $DRY_RUN_CMD mkdir -p ${lib.escapeShellArg (builtins.dirOf cfg.configFile)}

      # Create parent directories for all secrets
      ${lib.concatMapStrings (secret: ''
        $DRY_RUN_CMD mkdir -p "''${HOME}/${lib.escapeShellArg (builtins.dirOf secret.path)}"
      '') cfg.secrets}
    '';

    # Write secrets configuration with home-relative paths mapped to absolute paths
    home.activation.writeOpnixConfig = lib.hm.dag.entryAfter ["createOpnixDirs"] ''
      # Generate secrets configuration with expanded home paths
      $DRY_RUN_CMD cat > ${lib.escapeShellArg cfg.configFile} << 'EOF'
      {
        "secrets": ${builtins.toJSON (map (secret: {
          path = secret.path;
          reference = secret.reference;
        }) cfg.secrets)}
      }
      EOF
      $DRY_RUN_CMD chmod 600 ${lib.escapeShellArg cfg.configFile}
    '';

    # Retrieve secrets during activation
    home.activation.retrieveOpnixSecrets = lib.hm.dag.entryAfter ["writeOpnixConfig"] ''
      if [ ! -r ${lib.escapeShellArg cfg.tokenFile} ]; then
        echo "Error: Cannot read system token at ${cfg.tokenFile}" >&2
        echo "Make sure the system token can be accessed by your user." >&2
        exit 1
      fi

      # Retrieve secrets using system token
      $DRY_RUN_CMD ${pkgsWithOverlay.opnix}/bin/opnix secret \
        -token-file ${lib.escapeShellArg cfg.tokenFile} \
        -config ${lib.escapeShellArg cfg.configFile} \
        -output "$HOME"
    '';
  };
}
