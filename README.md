```
 ██████╗ ██████╗ ███╗   ██╗██╗██╗  ██╗
██╔═══██╗██╔══██╗████╗  ██║██║╚██╗██╔╝
██║   ██║██████╔╝██╔██╗ ██║██║ ╚███╔╝ 
██║   ██║██╔═══╝ ██║╚██╗██║██║ ██╔██╗ 
╚██████╔╝██║     ██║ ╚████║██║██╔╝ ██╗
 ╚═════╝ ╚═╝     ╚═╝  ╚═══╝╚═╝╚═╝  ╚═╝
```

# OPNix: 1Password Secrets for NixOS

Secure integration between 1Password and NixOS for managing secrets during system builds and home directory setup.

## Overview
```
╭────────────────────────────────────────────╮
│ • Secure secret storage in 1Password       │
│ • NixOS integration via service accounts   │
│ • Build-time secret retrieval             │
│ • Home Manager secret management          │
╰────────────────────────────────────────────╯
```

## Installation

Add OPNix to your NixOS configuration:

```nix
{
  inputs.opnix.url = "github:brizzbuzz/opnix";
  
  outputs = { self, nixpkgs, opnix }: {
    nixosConfigurations.yourhostname = nixpkgs.lib.nixosSystem {
      modules = [
        opnix.nixosModules.default
        ./configuration.nix
      ];
    };

    # If using nix-darwin
    darwinConfigurations.yourhostname = nix-darwin.lib.darwinSystem {
      modules = [
        opnix.darwinModules.default
        ./configuration.nix
      ];
    };

    # If using home-manager
    homeConfigurations.yourusername = home-manager.lib.homeManagerConfiguration {
      modules = [
        opnix.homeManagerModules.default
        ./home.nix
      ];
    };

  };
}
```

## Setup

1. Create a 1Password service account and generate a token:
   - Follow the [1Password documentation](https://developer.1password.com/docs/service-accounts/get-started)

2. Store the token securely:
   ```bash
   # Using the opnix CLI (recommended)
   sudo opnix token set
   
   # Or with a custom path
   sudo opnix token set -path /path/to/token
   ```

3. Create a secrets configuration file for system secrets:
   ```json
   {
     "secrets": [
       {
         "path": "mysql/root-password",
         "reference": "op://vault/database/root-password"
       },
       {
         "path": "ssl/private-key",
         "reference": "op://vault/certificates/private-key"
       },
       {
         "path": "ssl/cloudflare-cert",
         "reference": "op://Homelab/SSL Certificates/example.com/cert"
       }
     ]
   }
   ```

4. Enable OPNix in your NixOS configuration:
   ```nix
   {
     services.onepassword-secrets = {
       enable = true;
       users = [ "yourusername" ];  # Users that need secret access
       tokenFile = "/etc/opnix-token";  # Default location
       configFile = "/path/to/your/secrets.json";
       outputDir = "/var/lib/opnix/secrets";  # Optional, this is the default

       # Or use declarative secrets with camelCase variable names
       secrets = {
         databasePassword = {
           reference = "op://Vault/Database/password";
           services = ["postgresql"];
         };
         sslCertificate = {
           reference = "op://Vault/SSL/certificate";
           path = "/etc/ssl/certs/app.pem";
           owner = "caddy";
           group = "caddy";
           mode = "0644";
         };
       };

       # For darwin systems only:
       #
       # groupId = 600; 
       #
       # 600 is the default, but you should probably run
       # `dscl . list /Groups PrimaryGroupID | tr -s ' ' | sort -n -t ' ' -k2,2`
       # to find an unused gid.
     };
   }
   ```

5. (Optional) Set up Home Manager integration for user-specific secrets:
   ```nix
   {
     programs.onepassword-secrets = {
       enable = true;
       secrets = {
         sshPrivateKey = {
           # Paths are relative to home directory
           path = ".ssh/id_rsa";
           reference = "op://Personal/ssh-key/private-key";
         };
         secretAppToken = {
           path = ".config/secret-app/token";
           reference = "op://Work/api/token";
         };
       };
     };
   }
   ```

## Commands
```
╭─ CLI Commands ──────────────────────────────╮
│ opnix secret                               │
│ └─ Retrieve secrets from 1Password         │
│                                           │
│ opnix token set                           │
│ └─ Set up service account token           │
╰───────────────────────────────────────────╯
```

## Security Considerations

### Token Storage
- Store token file with proper permissions (600 for system, 640 for group access)
- Default location: `/etc/opnix-token`
- Never commit tokens to version control
- Access controlled via onepassword-secrets group for Home Manager users

### Service Account Security
- Use minimal required permissions
- Rotate tokens regularly
- Monitor service account activity

## Troubleshooting

Common issues and solutions:

1. Token File Issues:
   ```
   Warning: Token file not found
   ▪ OpNix will continue with existing secrets
   ▪ Run 'opnix token set' to configure the token
   ▪ System boot will NOT be affected
   ```

2. Authentication Problems:
   ```
   Error: Authentication failed
   ▪ Verify token validity
   ▪ Check service account permissions
   ▪ Service will retry automatically
   ```

3. Secret Access:
   ```
   Error: Cannot access secret
   ▪ Verify secret reference format
     ▪ Install 1password CLI and verify using op item get --format json
   ▪ Check service account vault access
   ```

### System Reliability

OpNix V1 uses systemd services (Linux) and launchd services (macOS) instead of activation scripts, ensuring:
- **System boot reliability**: Missing tokens will NOT cause unbootable systems
- **Automatic retry**: Services restart on failure and retry when tokens become available
- **Graceful degradation**: Continues with existing secrets when tokens are unavailable

## Development

For local development:
```bash
# Enter development shell
nix develop

# Run tests
go test ./...
```

## License

[MIT License](LICENSE)

## Credits
- Inspired by [agenix](https://github.com/ryantm/agenix)
- Built with [1Password SDK for Go](https://github.com/1Password/onepassword-sdk-go)
