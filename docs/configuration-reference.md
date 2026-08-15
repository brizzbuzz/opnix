# Configuration Reference

This document provides a comprehensive reference for all OpNix configuration options across NixOS, nix-darwin, and Home Manager.

## Table of Contents

- [NixOS/nix-darwin Configuration](#nixosnix-darwin-configuration)
- [Home Manager Configuration](#home-manager-configuration)
- [Common Options](#common-options)
- [Secret Path References](#secret-path-references)
- [Service Integration](#service-integration)
- [Advanced Configuration](#advanced-configuration)

## NixOS/nix-darwin Configuration

Configure OpNix using the `services.onepassword-secrets` module:

```nix
services.onepassword-secrets = {
  # ... options
};
```

### Core Options

#### `enable`
- **Type**: `bool`
- **Default**: `false`
- **Description**: Enable 1Password secrets integration

#### `tokenFile`
- **Type**: `path`
- **Default**: `"/etc/opnix-token"`
- **Description**: Path to file containing the 1Password service account token
- **Notes**: 
  - File should contain only the token
  - Recommended permissions: `640` (readable by root and opnix group)
  - Use `opnix token set` command to configure

**Example:**
```nix
services.onepassword-secrets = {
  enable = true;
  tokenFile = "/etc/opnix-token";
};
```

#### `configFiles`
- **Type**: `listOf path`
- **Default**: `[]`
- **Description**: List of JSON configuration files containing secrets
- **Notes**: Supports multiple config files for organization

**Example:**
```nix
services.onepassword-secrets = {
  configFiles = [
    ./database-secrets.json
    ./api-secrets.json
    ./ssl-secrets.json
  ];
};
```

#### `outputDir`
- **Type**: `str`
- **Default**: `"/var/lib/opnix/secrets"` (NixOS), `"/usr/local/var/opnix/secrets"` (nix-darwin)
- **Description**: Base directory where secrets are stored
- **Notes**: Used as fallback when secrets don't specify custom paths

#### `secretPaths`
- **Type**: `attrsOf str`
- **Default**: `{}`
- **Description**: Computed paths for declarative secrets (automatically populated)
- **Notes**: Values are runtime file paths, not secret values. OpNix creates the files during activation or service execution, after Nix evaluation has finished.

Use these paths only with consumers that read files at runtime. Do not pass an
OpNix path to `builtins.readFile`: pure evaluation cannot access the mutable
runtime file, and embedding its contents in evaluated configuration could copy
the secret into the Nix store, derivations, logs, or generation metadata.

#### `users` (NixOS only)
- **Type**: `listOf str`
- **Default**: `[]`
- **Description**: Users that should have access to secrets via group membership
- **Notes**: Users are added to the `onepassword-secrets` group

**Example:**
```nix
services.onepassword-secrets = {
  users = ["alice" "bob" "caddy"];
};
```

#### `groupId` (nix-darwin only)
- **Type**: `ints.between 500 1000`
- **Default**: `600`
- **Description**: Group ID for the `onepassword-secrets` group
- **Notes**: Must be an unused GID. Check existing groups with:
  ```bash
  dscl . list /Groups PrimaryGroupID | tr -s ' ' | sort -n -t ' ' -k2,2
  ```

### Declarative Secrets Configuration

#### `secrets`
- **Type**: `attrsOf secretOptions`
- **Default**: `{}`
- **Description**: Declarative secrets configuration using camelCase variable names as keys
- **Validation**: Keys must follow camelCase naming convention (e.g., `databasePassword`, not `"database/password"`)

**Example:**
```nix
services.onepassword-secrets.secrets = {
  example = {
    databasePassword = {
      reference = "op://Homelab/Database/password";
      services = ["postgresql"];
    };
    sslCertificate = {
      reference = "op://Homelab/SSL/certificate";
      path = "/etc/ssl/certs/app.pem";
      owner = "caddy";
      mode = "0644";
    };
  };
```

**Naming Rules:**
- Start with lowercase letter: `databasePassword` ✓, `DatabasePassword` ✗
- Use camelCase: `apiKey` ✓, `api_key` ✗, `api-key` ✗
- Alphanumeric only: `oauth2Token` ✓, `"oauth2-token"` ✗
- No quotes or special characters: `sslCert` ✓, `"ssl/cert"` ✗

### Secret Options

Each secret in the `secrets` attribute set supports these options:

#### `reference` (required)
- **Type**: `str`
- **Description**: 1Password reference in the format `op://Vault/Item/field` or `op://Vault/Item/Section/field`
- **Example**: `"op://Homelab/Database/password"` or `"op://Homelab/SSL Certs/example.com/cert"`

#### `path`
- **Type**: `nullOr str`
- **Default**: `null`
- **Description**: Custom absolute path for the secret file. If null, uses `outputDir + secret name`
- **Example**: `"/etc/ssl/certs/app.pem"`

#### `symlinks`
- **Type**: `listOf str`
- **Default**: `[]`
- **Description**: List of additional symlink paths that should point to this secret
- **Example**: `["/etc/ssl/certs/legacy.pem" "/opt/service/ssl/cert.pem"]`

#### `variables`
- **Type**: `attrsOf str`
- **Default**: `{}`
- **Description**: Variables for path template substitution
- **Example**: 
  ```nix
  variables = {
    service = "postgresql";
    environment = "prod";
  };
  ```

#### `owner`
- **Type**: `str`
- **Default**: `"root"`
- **Description**: User who owns the secret file
- **Example**: `"caddy"`

#### `group`
- **Type**: `str`
- **Default**: `"root"`
- **Description**: Group that owns the secret file
- **Example**: `"caddy"`

#### `mode`
- **Type**: `str`
- **Default**: `"0600"`
- **Description**: File permissions in octal notation
- **Example**: `"0644"`

#### `services`
- **Type**: `either (listOf str) (attrsOf serviceOptions)`
- **Default**: `[]`
- **Description**: Services to manage when this secret changes
- **Notes**: Can be a simple list of service names or detailed service configuration

**Simple list example:**
```nix
services = ["caddy" "postgresql"];
```

**Advanced configuration example (NixOS only):**
```nix
services = {
  caddy = {
    restart = true;
    after = ["opnix-secrets.service"];
  };
  backup-service = {
    restart = false;
    signal = "SIGHUP";
  };
};
```

**Note**: nix-darwin only supports simple service lists (not advanced configuration):
```nix
# nix-darwin - simple list only
services = ["com.example.myservice"];
```

### Service Options

When using advanced service configuration (NixOS only), each service supports:

#### `restart`
- **Type**: `bool`
- **Default**: `true`
- **Description**: Whether to restart the service when this secret changes

#### `signal`
- **Type**: `nullOr str`
- **Default**: `null`
- **Description**: Custom signal to send instead of restart (e.g., SIGHUP for reload)
- **Example**: `"SIGHUP"`

#### `after`
- **Type**: `listOf str`
- **Default**: `["opnix-secrets.service"]`
- **Description**: Additional systemd dependencies for this service

Prefer ordering managed services with `After=opnix-secrets.service` and `Wants=opnix-secrets.service`. Avoid `Requires=opnix-secrets.service` unless the service must fail hard when OpNix fails; hard requirements can deadlock if OpNix triggers a restart while the managed service is ordered after `opnix-secrets.service`.

`opnix-secrets.service` does not automatically restart after exit status `65`
(missing, invalid, or ambiguous references) or `75` (1Password rate limiting).
These conditions require configuration repair or waiting for the provider reset
window rather than repeated systemd starts.

### Path Template Configuration

#### `pathTemplate`
- **Type**: `str`
- **Default**: `""`
- **Description**: Template for generating secret paths with variable substitution
- **Variables**: `{service}`, `{environment}`, `{name}`, custom variables from `secrets.<name>.variables`
- **Example**: `"/etc/secrets/{service}/{environment}/{name}"`

#### `defaults`
- **Type**: `attrsOf str`
- **Default**: `{}`
- **Description**: Default values for template variables
- **Example**:
  ```nix
  defaults = {
    environment = "prod";
    service = "app";
  };
  ```

### systemd Integration

#### `systemdIntegration`
- **Type**: `systemdIntegrationOptions`
- **Default**: `{ enable = true; }`
- **Description**: Advanced systemd integration configuration

**Example:**
```nix
services.onepassword-secrets.systemdIntegration = {
  enable = true;
  services = ["caddy" "postgresql"];
  restartOnChange = true;
  changeDetection.enable = true;
  polling = {
    enable = true;
    interval = "6h";
  };
};
```

### systemd Integration Options

#### `enable`
- **Type**: `bool`
- **Default**: `true`
- **Description**: Enable advanced systemd integration features

#### `services`
- **Type**: `listOf str`
- **Default**: `[]`
- **Description**: Global list of services that depend on secrets
- **Example**: `["caddy" "postgresql" "grafana"]`

#### `restartOnChange`
- **Type**: `bool`
- **Default**: `true`
- **Description**: Automatically restart services when secrets change

#### `changeDetection`
- **Type**: `changeDetectionOptions`
- **Default**: `{}`
- **Description**: Configuration for secret change detection

##### `changeDetection.enable`
- **Type**: `bool`
- **Default**: `true`
- **Description**: Enable content-based change detection

##### `changeDetection.hashFile`
- **Type**: `str`
- **Default**: `"/var/lib/opnix/secret-hashes.json"`
- **Description**: File to store secret content hashes for change detection

#### `polling`
- **Type**: `pollingOptions`
- **Default**: `{}`
- **Description**: NixOS-only automatic polling for remote 1Password changes

##### `polling.enable`
- **Type**: `bool`
- **Default**: `false`
- **Description**: Enable periodic secret retrieval through `opnix-secrets-poll.timer`

##### `polling.interval`
- **Type**: `str`
- **Default**: `"6h"`
- **Description**: A systemd time span used for both the first poll after boot and subsequent polls
- **Example**: `"30min"`

The normal boot-time `opnix-secrets.service` still retrieves secrets immediately.
Polling uses a separate repeatable oneshot and the existing content hashes, so
unchanged values do not restart managed services. Failed polls are recorded in
journald and are retried at the next configured interval rather than in a tight
loop. The local path watcher is suspended during each poll and restored when it
finishes, preventing OpNix's own file operations from scheduling a duplicate
retrieval.

#### `errorHandling`
- **Type**: `errorHandlingOptions`
- **Default**: `{}`
- **Description**: Error handling and recovery configuration

##### `errorHandling.rollbackOnFailure`
- **Type**: `bool`
- **Default**: `false`
- **Description**: Reserved for future secret-file rollback handling; currently has no runtime effect
- **Notes**: This option does not roll back NixOS generations or preserve 1Password references used by older generations. Keep retired fields available through the rollback window instead.

##### `errorHandling.continueOnError`
- **Type**: `bool`
- **Default**: `true`
- **Description**: Continue processing other secrets if one fails

##### `errorHandling.maxRetries`
- **Type**: `int`
- **Default**: `3`
- **Description**: Maximum number of retry attempts for failed operations

## Home Manager Configuration

Configure OpNix using the `programs.onepassword-secrets` module:

```nix
programs.onepassword-secrets = {
  # ... options
};
```

### Home Manager Options

#### `enable`
- **Type**: `bool`
- **Default**: `false`
- **Description**: Enable 1Password secrets integration for Home Manager

#### `tokenFile`
- **Type**: `path`
- **Default**: `"/etc/opnix-token"`
- **Description**: Path to 1Password service account token file
- **Notes**: Can reference system token or use user-specific token

#### `configFiles`
- **Type**: `listOf path`
- **Default**: `[]`
- **Description**: List of JSON configuration files
- **Example**: `[./personal-secrets.json ./work-secrets.json]`

#### `secrets`
- **Type**: `attrsOf homeSecretOptions`
- **Default**: `{}`
- **Description**: Declarative secrets for Home Manager
- **Notes**: Relative paths are resolved from the home directory; absolute paths are used as-is

**Example:**
```nix
programs.onepassword-secrets.secrets = {
  sshPrivateKey = {
    reference = "op://Personal/SSH/private-key";
    path = ".ssh/id_rsa";
    mode = "0600";
  };
};
```

### Home Manager Secret Options

#### `reference` (required)
- **Type**: `str`
- **Description**: 1Password reference
- **Example**: `"op://Personal/SSH/private-key"`

#### `path`
- **Type**: `nullOr str`
- **Default**: `null`
- **Description**: Path for the secret file. Relative paths are resolved from the home directory; absolute paths are used as-is. If null, uses secret name
- **Example**: `".ssh/id_rsa"`

#### `owner`
- **Type**: `str`
- **Default**: `config.home.username`
- **Description**: File owner (defaults to home user)

#### `group`
- **Type**: `str`
- **Default**: `"users"`
- **Description**: File group

#### `mode`
- **Type**: `str`
- **Default**: `"0600"`
- **Description**: File permissions in octal notation

## Common Options

### JSON Configuration File Format

When using `configFiles`, each JSON file should follow this structure:

```json
{
  "secrets": [
    {
      "path": "relative/path/to/secret",
      "reference": "op://Vault/Item/field",
      "owner": "user",
      "group": "group", 
      "mode": "0600"
    },
    {
      "path": "ssl/certificate",
      "reference": "op://Vault/SSL Certs/example.com/cert",
      "owner": "caddy",
      "group": "caddy",
      "mode": "0644"
    }
  ]
}
```

**Required fields:**
- `path`: Relative path for the secret
- `reference`: 1Password reference

**Optional fields:**
- `owner`: File owner (default: "root" for system, username for Home Manager)
- `group`: File group (default: "root" for system, "users" for Home Manager)
- `mode`: File permissions (default: "0600")

### 1Password Reference Format

All 1Password references must follow the format:
```
op://VaultName/ItemName/FieldName
```

**Examples:**
- `op://Homelab/Database/password`
- `op://Personal/SSH-Keys/private-key`
- `op://Work/API-Tokens/github-token`

**Special fields:**
- `password`: The item's password field
- `username`: The item's username field
- `notes`: The item's notes field
- Custom field names as defined in 1Password

## Secret Path References

OpNix automatically generates path references that can be used in other parts of your configuration:

### System Configuration (NixOS)

```nix
services.onepassword-secrets = {
  enable = true;
  secrets = {
    sslCert = {
      reference = "op://Homelab/SSL/certificate";
      owner = "caddy";
      group = "caddy";
      mode = "0440";
      services = ["caddy"];
    };
    sslKey = {
      reference = "op://Homelab/SSL/private-key";
      owner = "caddy";
      group = "caddy";
      mode = "0400";
      services = ["caddy"];
    };
  };
};

# Caddy reads the certificate and key from these paths at runtime.
services.caddy = {
  enable = true;
  virtualHosts."example.com" = {
    extraConfig = ''
      tls ${config.services.onepassword-secrets.secretPaths.sslCert} ${config.services.onepassword-secrets.secretPaths.sslKey}
    '';
  };
};
```

### Home Manager Configuration

```nix
programs.onepassword-secrets = {
  enable = true;
  secrets.gitSigningKey = {
    reference = "op://Personal/Git/signing-key";
    path = ".ssh/git_signing_key";
    mode = "0600";
  };
};

# Git reads the private key from this path when it signs a commit.
programs.git = {
  enable = true;
  extraConfig = {
    gpg.format = "ssh";
    commit.gpgsign = true;
    user = {
      signingkey = config.programs.onepassword-secrets.secretPaths.gitSigningKey;
    };
  };
};
```

`secretPaths` cannot expose secret contents as Nix strings. Nix evaluates Home
Manager configuration before OpNix retrieves secrets, and evaluated strings are
often serialized into world-readable store paths. If an application requires an
inline value rather than a file path or runtime command, generate that
application's configuration outside the Nix store during activation or service
startup. OpNix does not currently provide runtime template rendering.

## Service Integration

OpNix can automatically manage systemd services when secrets change:

### Basic Service Integration

**NixOS:**
```nix
services.onepassword-secrets.secrets = {
  webSslCert = {
    reference = "op://Homelab/SSL/certificate";
    services = ["caddy" "nginx"];  # Restart these services when secret changes
  };
};
```

**nix-darwin:**
```nix
services.onepassword-secrets.secrets = {
  webSslCert = {
    reference = "op://Homelab/SSL/certificate";
    services = ["com.example.caddy"];  # macOS service identifiers
  };
};
```

### Advanced Service Integration (NixOS only)

```nix
services.onepassword-secrets.secrets = {
  databasePassword = {
    reference = "op://Homelab/Database/password";
    services = {
      postgresql = {
        restart = true;  # Full restart
        after = ["opnix-secrets.service"];
      };
      pgbouncer = {
        restart = false;  # Don't restart
        signal = "SIGHUP";  # Send reload signal instead
      };
    };
  };
};
```

**Note**: Advanced service configuration is only available on NixOS. nix-darwin uses simple service lists.

### Global Service Dependencies

Configure services to wait for secrets to be available:

```nix
services.onepassword-secrets.systemdIntegration = {
  enable = true;
  services = ["caddy" "postgresql" "grafana"];
  restartOnChange = true;
};
```

This automatically adds systemd dependencies so services wait for secrets to be deployed.

## Advanced Configuration

### Path Templates

Use templates to organize secrets systematically:

```nix
services.onepassword-secrets = {
  pathTemplate = "/etc/secrets/{service}/{environment}/{name}";
  defaults = {
    environment = "prod";
  };
  
  secrets = {
    databasePassword = {
      reference = "op://Homelab/Database/password";
      variables = {
        service = "postgresql";
      };
      # Results in: /etc/secrets/postgresql/prod/databasePassword
    };
  };
};
```

### Multiple Configuration Files

Organize secrets across multiple files:

```nix
services.onepassword-secrets = {
  configFiles = [
    ./secrets/database.json      # Database credentials
    ./secrets/api-keys.json      # API keys and tokens  
    ./secrets/ssl-certs.json     # SSL certificates
  ];
};
```

### Change Detection and Rollback Limits

Configure content-based change detection:

```nix
services.onepassword-secrets.systemdIntegration = {
  enable = true;
  changeDetection = {
    enable = true;
    hashFile = "/var/lib/opnix/secret-hashes";
  };
};
```

`rollbackOnFailure` is currently reserved and does not restore secret files or
roll back a NixOS generation. See
[Secret Reference Migrations and Rollback Safety](./migration-guide.md#secret-reference-migrations-and-rollback-safety)
before changing or deleting a 1Password field used by a deployed generation.

### Custom Token Locations

Use different token files for different environments:

```nix
services.onepassword-secrets = {
  tokenFile = "/run/secrets/opnix-token";
  # or
  tokenFile = "/home/user/.config/opnix/token";
};
```

## Development Shell Environments

OpNix can resolve 1Password secrets directly into environment variables for development tooling. This is useful for `nix develop` shells, CI jobs, or local scripting where writing secrets to disk is undesirable.

### Environment Configuration

Describe your environment in Nix (or provide JSON on the CLI):

```nix
{
  format = "shell";
  vars = [
    { name = "API_TOKEN"; reference = "op://Homelab/API/token"; }
    { name = "STATIC_VALUE"; value = "local-dev"; }
    { name = "OPTIONAL_PASSWORD"; reference = "op://Services/Database/password"; optional = true; }
  ];
}
```

#### Fields

- `vars` (required): Array of environment variable definitions.
  - `name` (required): Uppercase environment variable name.
  - `reference`: 1Password reference in the format `op://Vault/Item/field`.
  - `value`: Static fallback value when no reference is needed.
  - `optional`: Skip the variable when resolution fails instead of raising an error.
  - `preserveWhitespace`: Keep leading/trailing whitespace in the resolved value (defaults to trimming).
- `format` (optional): Preferred output format (`shell`, `dotenv`, or `json`). Can be overridden with the CLI flag.

### CLI Usage

Resolve environment variables on demand:

```bash
# Inline JSON for quick scripting
opnix env -config-json '{"vars":[{"name":"API_TOKEN","reference":"op://Homelab/API/token"}]}'

# Emit dotenv-compatible output
opnix env -config-json '{"vars":[{"name":"API_TOKEN","reference":"op://Homelab/API/token"}]}' -format dotenv

# Produce a JSON object
opnix env -config-json '{"vars":[{"name":"API_TOKEN","reference":"op://Homelab/API/token"}]}' -format json
```

The command reads tokens from `OP_SERVICE_ACCOUNT_TOKEN` or `-token-file` just like `opnix secret`. Static `value` entries do not require a token.

### Devshell Integration

The default OpNix devshell automatically evaluates `opnix env` when an environment configuration is provided.

```nix
let
  opnixEnvConfig = {
    vars = [
      { name = "API_TOKEN"; reference = "op://Homelab/DevShell API/token"; }
      { name = "STATIC_ENV"; value = "dev"; }
    ];
  };

  opnixEnvTokenFile =
    let tokenPath = builtins.getEnv "OPNIX_ENV_TOKEN_FILE";
    in if tokenPath == "" then null else tokenPath;
in {
  devShells.default = import ./nix/devshell.nix ({
    inherit pkgs buildOpnix opnixEnvConfig;
  } // pkgs.lib.optionalAttrs (opnixEnvTokenFile != null) {
    inherit opnixEnvTokenFile;
  });
}
```

Runtime behaviour can be tweaked with environment variables:

- `OPNIX_ENV_TOKEN_FILE`: Custom token path for devshell usage (defaults to `$HOME/.config/opnix/token` when unset).
- `OPNIX_ENV_DISABLE`: Any non-empty value skips secret resolution (useful for CI or offline work).

Recommended token workflow for devshells:

```bash
mkdir -p ~/.config/opnix
opnix token -path ~/.config/opnix/token set
chmod 600 ~/.config/opnix/token
export OPNIX_ENV_TOKEN_FILE=$HOME/.config/opnix/token
# or add the export to your shell profile/direnv
```

If the command succeeds, environment variables are exported via `eval` so subsequent shell commands can access them immediately. Errors are surfaced on stderr without terminating the shell.

## Validation and Assertions

OpNix automatically validates your configuration and provides helpful error messages:

- **File permissions**: Must be valid octal (e.g., "0644", "0600")
- **1Password references**: Must follow `op://Vault/Item/field` or `op://Vault/Item/Section/field` format
- **Path conflicts**: Prevents multiple secrets with the same output path
- **User/group existence**: Validates that specified users and groups exist
- **Configuration completeness**: Ensures at least one of `configFiles` or `secrets` is specified

## Security Considerations

### Token File Security
- Store tokens with restricted permissions (640 or 600)
- Never commit tokens to version control
- Use separate tokens for different environments
- Rotate tokens regularly

### Secret File Permissions
- Use restrictive permissions by default (0600)
- Only grant broader access when necessary (0644, 0640)
- Ensure parent directories have appropriate permissions
- Consider using dedicated users/groups for services

### Service Account Permissions
- Grant minimal required vault access
- Use separate service accounts for different environments
- Monitor service account activity
- Regularly audit vault access permissions

## Examples

See the [Examples](./examples/) directory for complete configuration examples covering common use cases.
