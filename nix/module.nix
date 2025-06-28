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

    configFile = lib.mkOption {
      type = lib.types.path;
      description = "Path to secrets configuration file";
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
      example = [ "alice" "bob" ];
  };

  config = lib.mkIf cfg.enable {
    # Create the opnix group
    users.groups.${opnixGroup} = {};

    # Add specified users to the opnix group
    users.users = lib.mkMerge (map (user: {
      ${user}.extraGroups = [ opnixGroup ];
    }) cfg.users);

    # Create systemd service instead of activation script
    systemd.services.opnix-secrets = {
      description = "OpNix Secret Management";
      wantedBy = [ "multi-user.target" ];
      after = [ "network.target" ];
      wants = [ "network.target" ];

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

        # Run the secrets retrieval tool
        ${pkgsWithOverlay.opnix}/bin/opnix secret \
          -token-file ${cfg.tokenFile} \
          -config ${cfg.configFile} \
          -output ${cfg.outputDir}
      '';
    };
  };
}
