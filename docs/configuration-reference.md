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
- **Notes**: This option provides declarative references to secret file paths for use in other configuration sections

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
- **Default**: `{}`
- **Description**: Advanced systemd integration configuration

**Example:**
```nix
services.onepassword-secrets.systemdIntegration = {
  enable = true;
  services = ["caddy" "postgresql"];
  restartOnChange = true;
  changeDetection.enable = true;
  errorHandling.rollbackOnFailure = true;
};
```

### systemd Integration Options

#### `enable`
- **Type**: `bool`
- **Default**: `false`
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
- **Default**: `"/var/lib/opnix/secret-hashes"`
- **Description**: File to store secret content hashes for change detection

#### `errorHandling`
- **Type**: `errorHandlingOptions`
- **Default**: `{}`
- **Description**: Error handling and recovery configuration

##### `errorHandling.rollbackOnFailure`
- **Type**: `bool`
- **Default**: `false`
- **Description**: Restore previous secrets on deployment failure

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
- **Notes**: Paths are relative to home directory

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
- **Description**: Path relative to home directory. If null, uses secret name
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

### System Configuration (NixOS/nix-darwin)

```nix
# Access secret paths in your configuration
services.postgresql = {
  enable = true;
  initialScript = pkgs.writeText "init.sql" ''
    ALTER USER postgres PASSWORD '$(cat ${config.services.onepassword-secrets.secretPaths.databasePassword})';
  '';
};

services.caddy = {
  enable = true;
  virtualHosts."example.com" = {
    tls = {
      cert = config.services.onepassword-secrets.secretPaths.sslCert;
      key = config.services.onepassword-secrets.secretPaths.sslKey;
    };
  };
};
```

### Home Manager Configuration

```nix
# Access secret paths in Home Manager
programs.git = {
  enable = true;
  extraConfig = {
    user = {
      signingkey = builtins.readFile config.programs.onepassword-secrets.secretPaths.gitSigningKey;
    };
  };
};
```

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

### Change Detection and Rollback

Enable advanced error handling:

```nix
services.onepassword-secrets.systemdIntegration = {
  enable = true;
  changeDetection = {
    enable = true;
    hashFile = "/var/lib/opnix/secret-hashes";
  };
  errorHandling = {
    rollbackOnFailure = true;
    continueOnError = false;
    maxRetries = 5;
  };
};
```

### Custom Token Locations

Use different token files for different environments:

```nix
services.onepassword-secrets = {
  tokenFile = "/run/secrets/opnix-token";
  # or
  tokenFile = "/home/user/.config/opnix/token";
};
```

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