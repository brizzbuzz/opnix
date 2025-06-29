# OpNix Documentation

Welcome to the comprehensive documentation for OpNix, the secure 1Password secrets integration for NixOS, nix-darwin, and Home Manager.

## What is OpNix?

OpNix provides seamless integration between 1Password and Nix-based systems for managing secrets during system builds and runtime. It securely retrieves secrets from 1Password using service accounts and deploys them to your NixOS, macOS (nix-darwin), or Home Manager configurations with proper permissions and service integration.

```
╭────────────────────────────────────────────╮
│ • Secure secret storage in 1Password       │
│ • NixOS integration via service accounts   │
│ • Build-time secret retrieval             │
│ • Home Manager secret management          │
│ • Automatic service restart on changes    │
│ • Cross-platform support (Linux/macOS)    │
╰────────────────────────────────────────────╯
```

## Key Features

- **Declarative Configuration**: Define secrets directly in Nix configuration
- **Flexible Ownership**: Per-secret user/group ownership and permissions
- **Custom Paths**: Absolute paths, path templates, and symlink support
- **Service Integration**: Automatic systemd/launchd service restarts on secret changes
- **Multi-Platform**: NixOS, nix-darwin, and Home Manager support
- **Reliable Architecture**: systemd/launchd services with graceful error handling
- **Security First**: Secure token management and file permissions

## Quick Start

### 1. Add OpNix to Your Flake

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    opnix.url = "github:brizzbuzz/opnix";
  };

  outputs = { nixpkgs, opnix, ... }: {
    nixosConfigurations.yourhostname = nixpkgs.lib.nixosSystem {
      modules = [
        opnix.nixosModules.default
        ./configuration.nix
      ];
    };
  };
}
```

### 2. Configure Your Secrets

```nix
services.onepassword-secrets = {
  enable = true;
  tokenFile = "/etc/opnix-token";
  
  secrets = {
    databasePassword = {
      reference = "op://Homelab/Database/password";
      owner = "postgres";
      services = ["postgresql"];
    };
  };
};
```

### 3. Set Up Your Token

```bash
sudo opnix token set
sudo nixos-rebuild switch --flake .
```

## Documentation Structure

### Getting Started
- **[Getting Started Guide](./getting-started.md)** - Complete setup walkthrough for all platforms
- **[Configuration Reference](./configuration-reference.md)** - Detailed reference for all options
- **[Migration Guide](./migration-guide.md)** - Upgrading from OpNix V0 to V1

### Guides
- **[Best Practices](./best-practices.md)** - Security, performance, and operational recommendations
- **[Troubleshooting](./troubleshooting.md)** - Common issues and debugging techniques

### Examples
- **[Examples Directory](./examples/)** - Real-world configuration examples
  - [Basic NixOS Setup](./examples/basic-nixos.md)
  - [Basic nix-darwin Setup](./examples/basic-darwin.md)
  - [Basic Home Manager Setup](./examples/basic-home-manager.md)
  - [Caddy Web Server](./examples/caddy-ssl.md)
  - [PostgreSQL Database](./examples/postgresql.md)
  - [And many more...](./examples/README.md)

## Platform Support

| Platform | Status | Module | Use Case |
|----------|--------|--------|----------|
| **NixOS** | ✅ Full Support | `nixosModules.default` | System-wide secret management |
| **nix-darwin** | ✅ Full Support | `darwinModules.default` | macOS system secret management |
| **Home Manager** | ✅ Full Support | `homeManagerModules.default` | User-specific secrets |

## Common Use Cases

### Web Services
```nix
# SSL certificates for web servers
services.onepassword-secrets.secrets.sslCert = {
  reference = "op://Homelab/SSL/certificate";
  path = "/etc/ssl/certs/app.pem";
  owner = "caddy";
  services = ["caddy"];
};
```

### Database Credentials
```nix
# Database passwords with service integration
services.onepassword-secrets.secrets.dbPassword = {
  reference = "op://Homelab/Database/password";
  owner = "postgres";
  services = ["postgresql"];
};
```

### API Keys and Tokens
```nix
# API keys for applications
services.onepassword-secrets.secrets.apiKey = {
  reference = "op://Homelab/API/key";
  owner = "myapp";
  mode = "0600";
};
```

### Home Manager Secrets
```nix
# User SSH keys and development tokens
programs.onepassword-secrets.secrets.sshKey = {
  reference = "op://Personal/SSH/private-key";
  path = ".ssh/id_rsa";
  mode = "0600";
};
```

## Architecture Overview

### V1 Architecture Improvements

OpNix V1 introduces significant reliability improvements:

- **systemd/launchd Services**: Replaced activation scripts with proper services
- **Graceful Degradation**: Missing tokens won't break system boot
- **Service Integration**: Automatic service dependencies and restart management
- **Change Detection**: Only update secrets when content actually changes
- **Error Recovery**: Rollback capabilities and comprehensive error handling

### Security Model

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   1Password     │    │      OpNix       │    │     Target      │
│   Service       │───▶│     Service      │───▶│    Services     │
│   Account       │    │   (systemd)      │    │  (postgres,     │
│                 │    │                  │    │   caddy, etc)   │
└─────────────────┘    └──────────────────┘    └─────────────────┘
        │                        │                        │
        │ Secure API            │ Managed Files         │ File Access
        │ Authentication        │ Proper Permissions    │ Service Restart
        │                       │                       │
        ▼                       ▼                       ▼
   Token stored              Secrets deployed       Services updated
   with restricted           with correct           automatically
   permissions               ownership
```

## What's New in V1?

### Major Features
- **Declarative Configuration**: Define secrets in Nix instead of JSON files
- **Flexible Ownership**: Per-secret user/group and permission control
- **Custom Paths**: Deploy secrets to any absolute path with template support
- **Service Integration**: Automatic systemd service dependencies and restarts
- **Enhanced Reliability**: systemd services with graceful error handling
- **Multi-Platform**: Full support for NixOS, nix-darwin, and Home Manager

### Backward Compatibility
V1 is fully backward compatible with V0 configurations. Your existing JSON-based configurations will continue to work while you gradually migrate to new features.

## Installation Methods

### With Flakes (Recommended)
```nix
inputs.opnix.url = "github:brizzbuzz/opnix";
```

### Without Flakes
```nix
let
  opnix = builtins.fetchTarball {
    url = "https://github.com/brizzbuzz/opnix/archive/main.tar.gz";
  };
in {
  imports = [ "${opnix}/nix/module.nix" ];
}
```

### Using Nix Channels
```bash
nix-channel --add https://github.com/brizzbuzz/opnix/archive/main.tar.gz opnix
nix-channel --update
```

## Getting Help

### Documentation
- **Read the guides** in this documentation directory
- **Check examples** for your specific use case
- **Review troubleshooting** for common issues

### Community Support
- **GitHub Issues**: [Report bugs and request features](https://github.com/brizzbuzz/opnix/issues)
- **GitHub Discussions**: [Ask questions and share configurations](https://github.com/brizzbuzz/opnix/discussions)

### Contributing
- **Documentation**: Help improve these guides
- **Examples**: Share your working configurations
- **Code**: Contribute features and bug fixes
- **Testing**: Help test new releases

## Security Notice

OpNix handles sensitive secrets and credentials. Please:

1. **Never commit tokens** or actual secrets to version control
2. **Use proper file permissions** (0600 for private keys, 0640 for shared access)
3. **Regularly rotate tokens** and monitor service account activity
4. **Follow security best practices** outlined in our guides
5. **Keep OpNix updated** to receive security fixes

## License

OpNix is released under the [MIT License](../LICENSE).

## Credits

- Inspired by [agenix](https://github.com/ryantm/agenix) for Nix secret management patterns
- Built with [1Password SDK for Go](https://github.com/1Password/onepassword-sdk-go)
- Thanks to the NixOS, nix-darwin, and Home Manager communities

---

**Ready to get started?** Begin with the [Getting Started Guide](./getting-started.md) or explore [Examples](./examples/) for your use case.