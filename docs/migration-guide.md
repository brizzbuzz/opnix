# Migration Guide: OpNix V0 to V1

This guide helps you migrate from OpNix V0 to V1, which introduces significant improvements including declarative configuration, flexible ownership, custom paths, and systemd service integration.

## Overview of Changes

### What's New in V1

- **Declarative Configuration**: Define secrets directly in Nix configuration
- **CamelCase Variable Names**: Idiomatic Nix variable naming (e.g., `databasePassword` instead of `"database/password"`)
- **Flexible Ownership**: Per-secret user/group ownership and permissions
- **Custom Paths**: Absolute paths and path templates
- **Service Integration**: Automatic service restarts on secret changes
- **systemd Services**: Reliable service-based architecture (no more activation scripts)
- **Enhanced Error Handling**: Graceful degradation and better error messages
- **Multi-platform Support**: NixOS, nix-darwin, and Home Manager modules

### Breaking Changes

- **CamelCase Variable Names Required**: Declarative secrets must use camelCase variable names (e.g., `databasePassword`, not `"database/password"`)
- **Activation Scripts → systemd Services**: OpNix now uses systemd services instead of activation scripts
- **New Module Structure**: Enhanced configuration options with better validation
- **Path Behavior**: Custom path handling with backward compatibility
- **Service Dependencies**: Automatic systemd service dependency management

### Backward Compatibility

**Good news**: V1 is fully backward compatible with V0 configurations. Your existing `configFile` setups will continue to work without changes.

## Migration Strategies

### Strategy 1: Keep Existing Configuration (Recommended for Quick Migration)

If your V0 configuration is working well, you can upgrade to V1 without changing anything:

```nix
# Your existing V0 configuration continues to work
services.onepassword-secrets = {
  enable = true;
  configFile = ./secrets.json;
  users = ["alice" "bob"];
  tokenFile = "/etc/opnix-token";
  outputDir = "/var/lib/opnix/secrets";
};
```

**Benefits:**
- Zero migration effort
- Immediate access to V1 reliability improvements
- No configuration changes required

**What you get:**
- systemd service reliability
- Graceful token handling
- Better error messages
- Improved boot safety

### Strategy 2: Gradual Migration to Declarative Configuration

Migrate incrementally by moving secrets from JSON files to declarative configuration:

```nix
services.onepassword-secrets = {
  enable = true;
  
  # Keep existing JSON files
  configFiles = [
    ./legacy-secrets.json
    ./database-secrets.json
  ];
  
  # Add new secrets declaratively
  secrets = {
    newServiceApiKey = {
      reference = "op://Vault/New-Service/api-key";
      owner = "new-service";
      mode = "0600";
    };
  };
};
```

**Benefits:**
- Gradual migration path
- Test new features incrementally
- Maintain existing working secrets

### Strategy 3: Full Migration to V1 Features

Completely migrate to the new declarative format and leverage all V1 features:

```nix
services.onepassword-secrets = {
  enable = true;
  tokenFile = "/etc/opnix-token";
  
  # All secrets defined declaratively
  secrets = {
    databasePassword = {
      reference = "op://Homelab/PostgreSQL/password";
      owner = "postgres";
      group = "postgres";
      mode = "0600";
      services = ["postgresql"];
    };
    
    sslCertificate = {
      reference = "op://Homelab/SSL/certificate";
      path = "/etc/ssl/certs/app.pem";
      owner = "caddy";
      group = "caddy";
      mode = "0644";
      services = {
        caddy = {
          restart = true;
          after = ["opnix-secrets.service"];
        };
      };
    };
  };
  
  # Enable advanced features
  systemdIntegration = {
    enable = true;
    services = ["caddy" "postgresql"];
    restartOnChange = true;
    changeDetection.enable = true;
  };
};
```

**Benefits:**
- Full access to V1 features
- Better configuration management
- Enhanced service integration
- Improved maintainability

## CamelCase Variable Names Migration

**Important**: Starting with OpNix V1, declarative secrets must use camelCase variable names instead of path-like strings.

### Before (V0 Style - No Longer Supported)
```nix
services.onepassword-secrets.secrets = {
  databasePassword = {
    reference = "op://Vault/Database/password";
  };
  sslCert = {
    reference = "op://Vault/SSL/certificate";
  };
  githubApiKey = {
    reference = "op://Personal/GitHub/token";
  };
};
```

### After (V1 Required Format)
```nix
services.onepassword-secrets.secrets = {
  databasePassword = {
    reference = "op://Vault/Database/password";
  };
  sslCert = {
    reference = "op://Vault/SSL/certificate";
  };
  githubApiKey = {
    reference = "op://Personal/GitHub/token";
  };
};
```

### Naming Convention Rules

1. **Start with lowercase letter**: `databasePassword` ✓, `DatabasePassword` ✗
2. **Use camelCase**: `apiKey` ✓, `api_key` ✗, `api-key` ✗
3. **No special characters**: `sslCert` ✓, `"ssl/cert"` ✗
4. **Alphanumeric only**: `oauth2Token` ✓, `oauth2-token` ✗

### Common Conversions

| Old Format | New Format |
|------------|------------|
| `"database/password"` | `databasePassword` |
| `"ssl/cert"` | `sslCert` |
| `"api-keys/github"` | `githubApiKey` |
| `"ssh/private-key"` | `sshPrivateKey` |
| `"config/app-token"` | `appConfigToken` |

### Migration Steps

1. **Identify all declarative secrets** in your configuration
2. **Convert keys to camelCase** following the naming rules
3. **Update any references** to the secret paths in your configuration
4. **Test the configuration** to ensure secrets are deployed correctly

### Path Behavior

The file paths remain the same - only the variable names change:

```nix
# Old format
sslCert = {
  reference = "op://Vault/SSL/certificate";
  path = "/etc/ssl/certs/app.pem";  # Explicit path
};

# New format - same behavior
sslCert = {
  reference = "op://Vault/SSL/certificate";
  path = "/etc/ssl/certs/app.pem";  # Same explicit path
};
```

When no explicit `path` is provided, the variable name is used directly:

```nix
# This creates a file at: /var/lib/opnix/secrets/databasePassword
databasePassword = {
  reference = "op://Vault/Database/password";
};
```

### Error Messages

If you use invalid variable names, OpNix will fail with a clear error:

```
error: Invalid secret key names. OpNix requires camelCase variable names like 'databasePassword', not path-like strings. Invalid keys: "database/password", "ssl/cert"
```

## Step-by-Step Migration

### Step 1: Update OpNix Version

Update your flake input to use OpNix V1:

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    opnix.url = "github:brizzbuzz/opnix";  # This will get latest V1
    # Or pin to specific version:
    # opnix.url = "github:brizzbuzz/opnix/v0.7.0";
  };
}
```

### Step 2: Test Current Configuration

Before making changes, ensure your current configuration works with V1:

```bash
# Build configuration without applying
nix build .#nixosConfigurations.yourhostname.config.system.build.toplevel

# Test the configuration
sudo nixos-rebuild test --flake .

# Check OpNix service status
sudo systemctl status opnix-secrets.service

# Verify secrets are accessible
ls -la /var/lib/opnix/secrets/
```

### Step 3: Choose Migration Path

Based on your needs, choose one of the migration strategies above.

### Step 4: Update Configuration

Implement your chosen migration strategy. Here are common patterns:

#### Converting JSON Configuration to Declarative

**Before (V0 JSON file):**
```json
{
  "secrets": [
    {
      "path": "databasePassword",
      "reference": "op://Homelab/Database/password"
    },
    {
      "path": "ssl/cert",
      "reference": "op://Homelab/SSL/certificate"
    }
  ]
}
```

**After (V1 Declarative):**
```nix
services.onepassword-secrets.secrets = {
  databasePassword = {
    reference = "op://Homelab/Database/password";
    # Add ownership and permissions
    owner = "postgres";
    group = "postgres";
    mode = "0600";
    services = ["postgresql"];
  };
  
  sslCert = {
    reference = "op://Homelab/SSL/certificate";
    # Use custom path
    path = "/etc/ssl/certs/app.pem";
    owner = "caddy";
    group = "caddy";
    mode = "0644";
    services = ["caddy"];
  };
};
```

#### Adding Service Integration

```nix
# Before: No service integration
services.onepassword-secrets = {
  enable = true;
  configFile = ./secrets.json;
};

# After: With service integration
services.onepassword-secrets = {
  enable = true;
  configFile = ./secrets.json;  # Keep existing
  
  # Add service integration
  systemdIntegration = {
    enable = true;
    services = ["caddy" "postgresql"];
    restartOnChange = true;
  };
};
```

### Step 5: Apply and Validate

```bash
# Apply the configuration
sudo nixos-rebuild switch --flake .

# Verify OpNix service is running
sudo systemctl status opnix-secrets.service

# Check service logs
sudo journalctl -u opnix-secrets.service

# Verify secrets are deployed correctly
sudo find /var/lib/opnix/secrets -type f -exec ls -la {} \;

# Test service integration (if enabled)
sudo systemctl restart opnix-secrets.service
sudo systemctl status postgresql.service caddy.service
```

## Platform-Specific Migration

### NixOS Migration

NixOS migration is straightforward since it was the primary platform for V0.

**Key changes:**
- Activation scripts → systemd services
- Enhanced error handling
- Service dependency management

**Validation:**
```bash
# Check systemd integration
systemctl list-dependencies opnix-secrets.service
systemctl show opnix-secrets.service -p After -p Before
```

### Adding nix-darwin Support

If you're adding macOS systems to your infrastructure:

```nix
# In your darwin configuration
services.onepassword-secrets = {
  enable = true;
  tokenFile = "/etc/opnix-token";
  groupId = 600;  # Choose unused GID
  users = ["yourusername"];
  
  secrets = {
    # Your secrets here
  };
};
```

**macOS-specific considerations:**
- Uses launchd instead of systemd
- Requires explicit group ID configuration
- Different default paths (`/usr/local/var/opnix/secrets`)

### Adding Home Manager Support

For user-specific secrets:

```nix
# In your home.nix
programs.onepassword-secrets = {
  enable = true;
  tokenFile = "/etc/opnix-token";  # Can use system token
  
  secrets = {
    sshPrivateKey = {
      reference = "op://Personal/SSH/private-key";
      path = ".ssh/id_rsa";
      mode = "0600";
    };
  };
};
```

## Common Migration Issues

### Issue 1: Service Startup Order

**Problem:** Services start before secrets are available.

**V0 Behavior:** No automatic dependency management.

**V1 Solution:**
```nix
services.onepassword-secrets.systemdIntegration = {
  enable = true;
  services = ["postgresql" "caddy"];  # These will wait for secrets
};
```

### Issue 2: Permission Problems

**Problem:** Services can't access secrets due to ownership.

**V0 Behavior:** All secrets owned by root with group access via `users` option.

**V1 Solution:**
```nix
services.onepassword-secrets.secrets = {
  databasePassword = {
    reference = "op://Vault/DB/password";
    owner = "postgres";  # Service-specific ownership
    group = "postgres";
    mode = "0600";
  };
};
```

### Issue 3: Boot Failures

**Problem:** System won't boot if token is missing.

**V0 Behavior:** Activation script failure could prevent boot.

**V1 Solution:** Automatic graceful degradation - missing tokens won't break boot.

```bash
# V1 handles missing tokens gracefully
WARNING: Token file /etc/opnix-token does not exist!
INFO: Using existing secrets, skipping updates
INFO: Run 'opnix token set' to configure the token
```

### Issue 4: Path Conflicts

**Problem:** Multiple secrets trying to write to same path.

**V1 Validation:** Configuration validation prevents conflicts.

```nix
# This will generate a build-time error
services.onepassword-secrets.secrets = {
  "secret1" = {
    reference = "op://Vault/Item1/field";
    path = "/etc/app/config";
  };
  "secret2" = {
    reference = "op://Vault/Item2/field";
    path = "/etc/app/config";  # Error: duplicate path
  };
};
```

## Advanced Migration Scenarios

### Multi-Environment Setup

If you have multiple environments (dev, staging, prod):

```nix
# Use environment-specific configuration
let
  environment = "prod";  # or "dev", "staging"
in {
  services.onepassword-secrets = {
    enable = true;
    tokenFile = "/etc/opnix-${environment}-token";
    
    secrets = {
      databasePassword = {
        reference = "op://Vault-${lib.toUpper environment}/Database/password";
        owner = "postgres";
        services = ["postgresql"];
      };
    };
  };
}
```

### Large-Scale Deployments

For deployments with many secrets:

```nix
services.onepassword-secrets = {
  enable = true;
  
  # Organize with multiple config files
  configFiles = [
    ./secrets/databases.json
    ./secrets/web-services.json
    ./secrets/monitoring.json
    ./secrets/backups.json
  ];
  
  # Enable advanced features for reliability
  systemdIntegration = {
    enable = true;
    changeDetection.enable = true;
    errorHandling = {
      rollbackOnFailure = true;
      continueOnError = false;
      maxRetries = 5;
    };
  };
};
```

### Home Manager Integration

Adding user-level secrets to existing system setup:

```nix
# System configuration (unchanged)
services.onepassword-secrets = {
  enable = true;
  secrets = {
    # System secrets
  };
};

# User configuration (new)
programs.onepassword-secrets = {
  enable = true;
  tokenFile = "/etc/opnix-token";  # Reuse system token
  
  secrets = {
    sshKey = {
      reference = "op://Personal/SSH/key";
      path = ".ssh/id_rsa";
      mode = "0600";
    };
  };
};
```

## Validation and Testing

### Pre-Migration Testing

```bash
# Test configuration syntax
nix-instantiate --eval --strict -E '
  with import <nixpkgs> {};
  (import ./configuration.nix { inherit pkgs; }).services.onepassword-secrets
'

# Test 1Password references
op item get "Database" --vault "Homelab" --format json
```

### Post-Migration Validation

```bash
# Comprehensive validation script
#!/bin/bash

echo "=== OpNix V1 Migration Validation ==="

# Check service status
echo "Checking OpNix service..."
systemctl is-active opnix-secrets.service || {
  echo "ERROR: OpNix service not running"
  exit 1
}

# Check secret files
echo "Checking secret files..."
find /var/lib/opnix/secrets -type f | while read -r file; do
  if [ ! -r "$file" ]; then
    echo "ERROR: Cannot read $file"
  else
    echo "OK: $file"
  fi
done

# Check service dependencies
echo "Checking service dependencies..."
for service in postgresql caddy nginx; do
  if systemctl is-enabled "$service" >/dev/null 2>&1; then
    if systemctl show "$service" -p After | grep -q opnix-secrets; then
      echo "OK: $service waits for opnix-secrets"
    else
      echo "WARN: $service might not wait for secrets"
    fi
  fi
done

# Check token
echo "Checking token..."
if [ -r /etc/opnix-token ]; then
  echo "OK: Token file accessible"
else
  echo "ERROR: Token file not accessible"
fi

echo "=== Validation Complete ==="
```

## Rollback Procedures

If you need to rollback to V0:

### Emergency Rollback

```bash
# Quick rollback using previous generation
sudo nixos-rebuild switch --rollback

# Or specify previous generation
sudo nixos-rebuild switch --switch-generation 123
```

### Planned Rollback

```nix
# In your flake.nix, pin to V0 version
inputs.opnix.url = "github:brizzbuzz/opnix/v0.6.0";

# Restore V0 configuration format
services.onepassword-secrets = {
  enable = true;
  configFile = ./secrets.json;
  users = ["alice"];
  tokenFile = "/etc/opnix-token";
};
```

## Getting Help

If you encounter issues during migration:

1. **Check the logs**: `sudo journalctl -u opnix-secrets.service`
2. **Validate configuration**: Use the validation scripts above
3. **Test incrementally**: Migrate one secret at a time
4. **Use staging environment**: Test changes before production
5. **Ask for help**: Open an issue on [GitHub](https://github.com/brizzbuzz/opnix/issues)

## Next Steps

After successful migration:

1. **Read the [Best Practices Guide](./best-practices.md)** for security and operational recommendations
2. **Explore [Examples](./examples/)** for advanced configuration patterns
3. **Set up monitoring** for OpNix services
4. **Plan regular updates** to stay current with OpNix releases
5. **Consider Home Manager integration** for user-specific secrets

The migration to OpNix V1 provides significant reliability and usability improvements. Take your time with the migration and test thoroughly in non-production environments first.
