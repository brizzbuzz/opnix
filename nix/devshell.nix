{
  pkgs,
  buildOpnix,
  opnixEnvConfig ? null,
  opnixEnvTokenFile ? null,
}: let
  lib = pkgs.lib;
  configPath =
    if opnixEnvConfig == null
    then null
    else toString opnixEnvConfig;
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
      ${lib.optionalString (configPath != null) ''
        if [ -z "''${OPNIX_ENV_CONFIG:-}" ]; then
          export OPNIX_ENV_CONFIG=${lib.escapeShellArg configPath}
        fi
      ''}

      ${lib.optionalString (tokenPath != null) ''
        if [ -z "''${OPNIX_ENV_TOKEN_FILE:-}" ]; then
          export OPNIX_ENV_TOKEN_FILE=${lib.escapeShellArg tokenPath}
        fi
      ''}

      if [ -n "''${OPNIX_ENV_DISABLE:-}" ]; then
        echo "INFO: OPNIX_ENV_DISABLE set, skipping opnix env exports" >&2
      else
        if [ -n "''${OPNIX_ENV_CONFIG:-}" ]; then
          if [ ! -f "''${OPNIX_ENV_CONFIG}" ]; then
            echo "WARNING: opnix env config not found at ''${OPNIX_ENV_CONFIG}" >&2
          else
            token_args=()
            if [ -n "''${OPNIX_ENV_TOKEN_FILE:-}" ]; then
              token_args=(-token-file "''${OPNIX_ENV_TOKEN_FILE}")
            fi

            if output="$(${buildOpnix}/bin/opnix env -config "''${OPNIX_ENV_CONFIG}" -format shell "''${token_args[@]}")"; then
              eval "$output"
            else
              echo "WARNING: failed to resolve opnix environment variables" >&2
            fi
          fi
        fi
      fi
    '';
  }
