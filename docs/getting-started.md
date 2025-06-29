# Getting Started with OpNix

OpNix provides secure integration between 1Password and NixOS/nix-darwin/Home Manager for managing secrets during system builds and runtime. This guide will walk you through setting up OpNix on your system.

## Overview

OpNix retrieves secrets from 1Password using service accounts and deploys them securely to your NixOS, macOS (nix-darwin), or Home Manager configurations. It supports:

- **NixOS**: System-wide secret management with systemd integration
- **nix-darwin**: macOS system secret management with launchd integration  
- **Home Manager**: User-specific secret management across platforms

## Prerequisites

1. **1Password account** with a service account
2. **NixOS**, **nix-darwin**, or **Home Manager** setup
3. **Flakes enabled** (recommended but not required)

## Quick Start

### Step 1: Add OpNix to Your Flake

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    opnix.url = "github:brizzbuzz/opnix";
    # ... your other inputs
  };

  outputs = { self, nixpkgs, opnix, ... }: {
    # NixOS configuration
    nixosConfigurations.yourhostname = nixpkgs.lib.nixosSystem {
      modules = [
        opnix.nixosModules.default
        ./configuration.nix
      ];
    };

    # macOS (nix-darwin) configuration
    darwinConfigurations.yourhostname = nix-darwin.lib.darwinSystem {
      modules = [
        opnix.darwinModules.default
        ./configuration.nix
      ];
    };

    # Home Manager configuration
    homeConfigurations.yourusername = home-manager.lib.homeManagerConfiguration {
      modules = [
        opnix.homeManagerModules.default
        ./home.nix
      ];
    };
  };
}
```

### Step 2: Set Up 1Password Service Account

1. **Create a service account** in your 1Password account:
   - Go to Developer Settings in your 1Password account
   - Create a new service account
   - Note the service account token

2. **Grant vault access** to the service account:
   - Give the service account read access to vaults containing your secrets
   - Test access using the 1Password CLI: `op item list --vault YourVault`

### Step 3: Configure Your Secrets

Choose your configuration method based on your setup:

#### Option A: Declarative Configuration (Recommended)

**NixOS/nix-darwin:**
```nix
services.onepassword-secrets = {
  enable = true;
  tokenFile = "/etc/opnix-token";
  
  secrets = {
    databasePassword = {
      reference = "op://Homelab/Database/password";
      owner = "postgres";
      group = "postgres";
      mode = "0600";
    };
    
    sslCertificate = {
      reference = "op://Homelab/SSL/certificate";
      path = "/etc/ssl/certs/app.pem";
      owner = "caddy";
      mode = "0644";
      services = ["caddy"];
    };
  };
};
```

**Home Manager:**
```nix
programs.onepassword-secrets = {
  enable = true;
  tokenFile = "/etc/opnix-token";
  
  secrets = {
    sshPrivateKey = {
      reference = "op://Personal/SSH/private-key";
      path = ".ssh/id_rsa";
      mode = "0600";
    };
    
    configApiKey = {
      reference = "op://Work/API/key";
      path = ".config/myapp/api-key";
      mode = "0600";
    };
  };
};
```

#### Option B: JSON Configuration Files

Create a secrets configuration file:

```json
{
  "secrets": [
    {
      "path": "databasePassword",
      "reference": "op://Homelab/Database/password"
    },
    {
      "path": "sslCertificate", 
      "reference": "op://Homelab/SSL/certificate"
    }
  ]
}
```

Then reference it in your configuration:

```nix
services.onepassword-secrets = {
  enable = true;
  tokenFile = "/etc/opnix-token";
  configFiles = [ ./secrets.json ];
};
```

### Step 4: Set Up the Service Account Token

**Install OpNix** first (it will be available after rebuilding):

```bash
# Rebuild your system to get the opnix binary
sudo nixos-rebuild switch --flake .
# or for nix-darwin:
darwin-rebuild switch --flake .
# or for Home Manager:
home-manager switch --flake .
```

**Set the token** using the OpNix CLI:

```bash
# Set token interactively (recommended)
sudo opnix token set

# Or set token from environment variable
export OP_SERVICE_ACCOUNT_TOKEN="your-token-here"
sudo opnix token set

# Or set token with custom path
sudo opnix token set -path /custom/path/to/token
```

### Step 5: Deploy Your Configuration

**Rebuild your system:**

```bash
# NixOS
sudo nixos-rebuild switch --flake .

# nix-darwin
darwin-rebuild switch --flake .

# Home Manager
home-manager switch --flake .
```

**Verify secrets are deployed:**

```bash
# Check system secrets (NixOS/nix-darwin)
ls -la /var/lib/opnix/secrets/  # Linux
ls -la /usr/local/var/opnix/secrets/  # macOS

# Check Home Manager secrets
ls -la ~/.ssh/
ls -la ~/.config/myapp/
```

## Platform-Specific Notes

### NixOS

- Uses **systemd services** for reliable secret management
- Secrets stored in `/var/lib/opnix/secrets/` by default
- Supports service integration and restart on secret changes
- Missing tokens won't break system boot (graceful degradation)

### macOS (nix-darwin)

- Uses **launchd services** for secret management
- Secrets stored in `/usr/local/var/opnix/secrets/` by default
- Requires explicit group ID configuration (`groupId` option)
- Users must be added to the `onepassword-secrets` group

### Home Manager

- Works on **any platform** (Linux, macOS, etc.)
- Secrets stored relative to home directory
- Runs during Home Manager activation
- Can access system tokens or use separate token files

## Common Patterns

### Web Server SSL Certificates

```nix
services.onepassword-secrets.secrets = {
  sslCert = {
    reference = "op://Homelab/SSL/certificate";
    path = "/etc/ssl/certs/app.pem";
    owner = "caddy";
    group = "caddy";
    mode = "0644";
    services = ["caddy"];
  };
  
  sslKey = {
    reference = "op://Homelab/SSL/private-key";
    path = "/etc/ssl/private/app.key";
    owner = "caddy";
    group = "caddy";
    mode = "0600";
    services = ["caddy"];
  };
};
```

### Database Credentials

```nix
services.onepassword-secrets.secrets = {
  postgresPassword = {
    reference = "op://Homelab/Database/password";
    owner = "postgres";
    group = "postgres";
    services = ["postgresql"];
  };
};

# Reference the secret in your PostgreSQL config
services.postgresql = {
  enable = true;
  authentication = "local all all trust";
  initialScript = pkgs.writeText "init.sql" ''
    ALTER USER postgres PASSWORD '$(cat ${config.services.onepassword-secrets.secretPaths.postgresPassword})';
  '';
};
```

### API Keys for Services

```nix
services.onepassword-secrets.secrets = {
  grafanaSecretKey = {
    reference = "op://Homelab/Grafana/secret-key";
    owner = "grafana";
    group = "grafana";
    services = ["grafana"];
  };
};

services.grafana = {
  enable = true;
  settings.security.secret_key = "$__file{${config.services.onepassword-secrets.secretPaths.grafanaSecretKey}}";
};
```

## Troubleshooting

### Token Issues

**Token file not found:**
```
WARNING: Token file /etc/opnix-token does not exist!
INFO: Using existing secrets, skipping updates
INFO: Run 'opnix token set' to configure the token
```

**Solution:** Run `sudo opnix token set` to configure the token.

**Authentication failed:**
```
ERROR: Authentication failed
INFO: Token may be expired or invalid
```

**Solution:** 
1. Verify the token in 1Password
2. Regenerate the service account token if needed
3. Run `sudo opnix token set` with the new token

### Permission Issues

**Cannot write secret file:**
```
ERROR: Cannot write secret file
Secret: sslCert
Target: /etc/ssl/certs/app.pem
Issue: Permission denied
```

**Solution:**
1. Create the target directory: `sudo mkdir -p /etc/ssl/certs`
2. Check parent directory permissions: `ls -la /etc/ssl/`
3. Ensure OpNix service has write access

### Secret Reference Issues

**Secret not found:**
```
ERROR: 1Password reference not found
Secret: apiKey
Reference: op://Vault/Missing-Item/field
Issue: Item 'Missing-Item' not found in vault 'Vault'
```

**Solution:**
1. Verify the secret exists in 1Password
2. Test the reference with 1Password CLI: `op item get "Missing-Item" --vault "Vault"`
3. Check service account vault access permissions

## Next Steps

- Read the [Configuration Reference](./configuration-reference.md) for detailed option documentation
- Check out [Examples](./examples/) for more complex configurations  
- See [Best Practices](./best-practices.md) for security recommendations
- Review [Migration Guide](./migration-guide.md) if upgrading from V0

## Getting Help

- **GitHub Issues**: [Report bugs and request features](https://github.com/brizzbuzz/opnix/issues)
- **Documentation**: Check the full documentation in the `docs/` directory
- **1Password CLI**: Use `op --help` for 1Password CLI documentation