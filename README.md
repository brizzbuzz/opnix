```
 ██████╗ ██████╗ ███╗   ██╗██╗██╗  ██╗
██╔═══██╗██╔══██╗████╗  ██║██║╚██╗██╔╝
██║   ██║██████╔╝██╔██╗ ██║██║ ╚███╔╝ 
██║   ██║██╔═══╝ ██║╚██╗██║██║ ██╔██╗ 
╚██████╔╝██║     ██║ ╚████║██║██╔╝ ██╗
 ╚═════╝ ╚═╝     ╚═╝  ╚═══╝╚═╝╚═╝  ╚═╝
```

# OPNix: 1Password Secrets for NixOS

Secure integration between 1Password and NixOS for managing secrets during system builds.

## Overview
```
╭────────────────────────────────────────────╮
│ • Secure secret storage in 1Password       │
│ • NixOS integration via service accounts   │
│ • Build-time secret retrieval             │
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
  };
}
```

## Setup

1. Create a 1Password service account and generate a token:
   - Follow the [1Password documentation](https://developer.1password.com/docs/service-accounts/get-started)

2. Store the token securely:
   ```bash
   # Using the opnix CLI (recommended)
   opnix token set
   
   # Or with a custom path
   opnix token set -path /path/to/token
   ```

3. Create a secrets configuration file:
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
       tokenFile = "/etc/opnix-token";  # Default location
       configFile = "/path/to/your/secrets.json";
       outputDir = "/var/lib/opnix/secrets";  # Optional, this is the default
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
- Store token file with proper permissions (600)
- Default location: `/etc/opnix-token`
- Never commit tokens to version control

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
