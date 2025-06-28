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

    configFile = lib.mkOption {
      type = lib.types.path;
      description = "Path to secrets configuration file";
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
  };

  config = lib.mkIf (cfg.enable) {
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

            # Run the secrets retrieval tool
            ${pkgsWithOverlay.opnix}/bin/opnix secret \
              -token-file ${cfg.tokenFile} \
              -config ${cfg.configFile} \
              -output ${cfg.outputDir}
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
  };
}
