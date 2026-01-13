{
  pkgs,
  buildOpnix,
  opnixEnvConfig ? null,
  opnixEnvTokenFile ? null,
}: let
  lib = pkgs.lib;
  configArg =
    if opnixEnvConfig == null
    then null
    else if builtins.isAttrs opnixEnvConfig
    then {
      flag = "-config-json";
      value = lib.escapeShellArg (builtins.toJSON opnixEnvConfig);
    }
    else {
      flag = "-config";
      value = lib.escapeShellArg (toString opnixEnvConfig);
    };
  tokenPath =
    if opnixEnvTokenFile == null
    then null
    else toString opnixEnvTokenFile;
in
  pkgs.mkShell {
    buildInputs = with pkgs; [
      alejandra
      just
      go
      gopls
      gotools
      go-tools
      golangci-lint
      nil
      buildOpnix
    ];

    shellHook = ''
      ${lib.optionalString (tokenPath != null) ''
        if [ -z "''${OPNIX_ENV_TOKEN_FILE:-}" ]; then
          export OPNIX_ENV_TOKEN_FILE=${lib.escapeShellArg tokenPath}
        fi
      ''}

      if [ -n "''${OPNIX_ENV_DISABLE:-}" ]; then
        echo "INFO: OPNIX_ENV_DISABLE set, skipping opnix env exports" >&2
      elif [ ${
        if configArg == null
        then "0"
        else "1"
      } -eq 0 ]; then
        echo "WARNING: no opnix environment configuration provided" >&2
      else
        token_args=()
        if [ -z "''${OPNIX_ENV_TOKEN_FILE:-}" ]; then
          export OPNIX_ENV_TOKEN_FILE="''${HOME}/.config/opnix/token"
        fi
        if [ -n "''${OPNIX_ENV_TOKEN_FILE:-}" ]; then
          token_args=(-token-file "''${OPNIX_ENV_TOKEN_FILE}")
        fi

        if output="$(${buildOpnix}/bin/opnix env ${configArg.flag} ${configArg.value} -format shell "''${token_args[@]}")"; then
          eval "$output"
        else
          echo "WARNING: failed to resolve opnix environment variables" >&2
        fi
      fi
    '';
  }
