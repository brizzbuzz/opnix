# Development Shell Environment Variables

This example configures OpNix to inject 1Password secrets as environment variables whenever you enter a flake devshell.

## Prerequisites

- OpNix added as a flake input
- Service account token stored in `/etc/opnix-token` or exported via `OP_SERVICE_ACCOUNT_TOKEN`
- `nix develop` (or `direnv` with flakes) available on your machine

## 1Password Setup

Create the secrets you want to expose in your development shell. For example:

- `op://Homelab/API/token` for API access
- `op://Homelab/Database/password` for optional local testing

## Environment Configuration

Describe the desired variables in Nix so the configuration lives alongside the flake:

```nix
let
  opnixEnvConfig = pkgs.writeText "opnix-env.json" (builtins.toJSON {
    vars = [
      { name = "API_TOKEN"; reference = "op://Homelab/API/token"; }
      { name = "LOCAL_DB_PASSWORD"; reference = "op://Homelab/Database/password"; optional = true; }
      { name = "STATIC_ENV"; value = "dev"; }
    ];
  });
in
```

- `API_TOKEN` must resolve successfully or the command exits with an error.
- `LOCAL_DB_PASSWORD` is optional; if it cannot be resolved a warning is emitted.
- `STATIC_ENV` demonstrates mixing in literal values.

## Flake Devshell Configuration

Wire the config into your flake:

```nix
{
  description = "Example devshell with OpNix env integration";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    opnix.url = "github:brizzbuzz/opnix";
  };

  outputs = { self, nixpkgs, opnix, ... }:
    let
      system = "x86_64-linux";
      pkgs = import nixpkgs { inherit system; };
      buildOpnix = opnix.packages.${system}.default;
      opnixEnvConfig = pkgs.writeText "opnix-env.json" (builtins.toJSON {
        vars = [
          { name = "API_TOKEN"; reference = "op://Homelab/API/token"; }
          { name = "LOCAL_DB_PASSWORD"; reference = "op://Homelab/Database/password"; optional = true; }
        ];
      });
      opnixEnvTokenFile =
        let tokenPath = builtins.getEnv "OPNIX_ENV_TOKEN_FILE";
        in if tokenPath == "" then null else tokenPath;
    in {
      devShells.${system}.default = import ./nix/devshell.nix {
        inherit pkgs buildOpnix;
        inherit opnixEnvConfig;
        opnixEnvTokenFile = opnixEnvTokenFile;
      };
    };
}
```

The bundled `nix/devshell.nix`:

1. Sets `OPNIX_ENV_CONFIG` to the provided path.
2. Calls `opnix env -format shell` when the shell starts.
3. `eval`s the command output to export environment variables.

## Usage

```bash
nix develop
# Shell exports are now available
echo "$API_TOKEN"
# -> secret resolved from 1Password
```

### Runtime Controls

- `OPNIX_ENV_DISABLE=1 nix develop` – skip secret resolution (useful offline or in CI).
- `OPNIX_ENV_TOKEN_FILE=/path/to/token nix develop` – override the token path.
- `OP_SERVICE_ACCOUNT_TOKEN=... nix develop` – use an in-memory token instead of a file.
- If unset, the flake defaults `OPNIX_ENV_TOKEN_FILE` to `$HOME/.config/opnix/token`.

### Alternative Formats

Need a `.env` file?

```bash
opnix env -config "$OPNIX_ENV_CONFIG" -format dotenv > .env
```

Or a JSON blob for scripting:

```bash
opnix env -config "$OPNIX_ENV_CONFIG" -format json | jq .
```

## Recommended Token Setup

```bash
mkdir -p ~/.config/opnix
opnix token -path ~/.config/opnix/token set
chmod 600 ~/.config/opnix/token
export OPNIX_ENV_TOKEN_FILE=$HOME/.config/opnix/token
# Add the export to your shell profile or .envrc for convenience
```

## Troubleshooting

- `WARNING: opnix env config not found` – verify `OPNIX_ENV_CONFIG` points to an existing file.
- `failed to resolve opnix environment variables` – check token access or references.
- Optional variables emit warnings but never abort the shell.

## Next Steps

- Combine with [direnv](https://direnv.net/) to automatically load secrets when entering the project directory.
- Use multiple environment configs for staging/production by switching `opnixEnvConfig`.
- Share the same configuration with CI by running `opnix env` directly in pipeline steps.
