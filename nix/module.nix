{ config, lib, pkgs, ... }: let
  cfg = config.services.onepassword-secrets;
  # Create a new pkgs instance with our overlay
  pkgsWithOverlay = import pkgs.path {
    inherit (pkgs) system;
    overlays = [
      (final: prev: {
        opnix = import ./package.nix { pkgs = final; };
      })
    ];
  };
in {
  options.services.onepassword-secrets = {
    enable = lib.mkEnableOption "1Password secrets integration";

    tokenFile = lib.mkOption {
      type = lib.types.path;
      default = "/etc/opnix-token";
      description = ''
        Path to file containing the 1Password service account token.
        The file should contain only the token and should have appropriate permissions (600).

        You can set up the token using the opnix CLI:
          opnix token set
          # or with a custom path:
          opnix token set -path /path/to/token
      '';
    };

    configFile = lib.mkOption {
      type = lib.types.path;
      description = "Path to secrets configuration file";
    };

    outputDir = lib.mkOption {
      type = lib.types.str;
      default = "/var/lib/opnix/secrets";
      description = "Directory to store retrieved secrets";
    };
  };

  config = lib.mkIf cfg.enable {
    system.activationScripts.onepassword-secrets = {
      deps = [];
      text = ''
        # Ensure output directory exists with correct permissions
        mkdir -p ${cfg.outputDir}
        chmod 750 ${cfg.outputDir}

        # Validate token file existence and permissions
        if [ ! -f ${cfg.tokenFile} ]; then
          echo "Error: Token file ${cfg.tokenFile} does not exist!" >&2
          exit 1
        fi

        if [ ! -r ${cfg.tokenFile} ]; then
          echo "Error: Token file ${cfg.tokenFile} is not readable!" >&2
          exit 1
        fi

        # Validate token is not empty (without printing content or length)
        if [ ! -s ${cfg.tokenFile} ]; then
          echo "Error: Token file is empty!" >&2
          exit 1
        fi

        # Run the secrets retrieval tool with new command structure
        ${pkgsWithOverlay.opnix}/bin/opnix secret \
          -token-file ${cfg.tokenFile} \
          -config ${cfg.configFile} \
          -output ${cfg.outputDir}
      '';
    };
  };
}
