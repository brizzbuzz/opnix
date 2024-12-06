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
     };
   }
   ```

5. (Optional) Set up Home Manager integration for user-specific secrets:
   ```nix
   {
     programs.onepassword-secrets = {
       enable = true;
       secrets = [
         {
           # Paths are relative to home directory
           path = ".ssh/id_rsa";
           reference = "op://Personal/ssh-key/private-key"
         }
         {
           path = ".config/secret-app/token";
           reference = "op://Work/api/token"
         }
       ];
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
   Error: Token file not found
   ▪ Check if /etc/opnix-token exists
   ▪ Verify file permissions
   ▪ For Home Manager, ensure user in onepassword-secrets group
   ```

2. Authentication Problems:
   ```
   Error: Authentication failed
   ▪ Verify token validity
   ▪ Check service account permissions
   ```

3. Secret Access:
   ```
   Error: Cannot access secret
   ▪ Verify secret reference format
   ▪ Check service account vault access
   ```

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
