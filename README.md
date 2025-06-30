# OpNix

Secure 1Password secrets integration for NixOS, nix-darwin, and Home Manager.

## Features

- **Declarative Secrets**: Define secrets directly in Nix configuration
- **Service Integration**: Automatic systemd/launchd service restarts on secret changes
- **Multi-Platform**: Full support for NixOS, nix-darwin, and Home Manager
- **Secure**: Uses 1Password service accounts with proper file permissions
- **Reliable**: systemd services ensure secrets are available without breaking system boot

## Quick Start

Add OpNix to your flake:

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    opnix.url = "github:brizzbuzz/opnix";
  };

  outputs = { nixpkgs, opnix, ... }: {
    # NixOS
    nixosConfigurations.yourhostname = nixpkgs.lib.nixosSystem {
      modules = [
        opnix.nixosModules.default
        ./configuration.nix
      ];
    };

    # nix-darwin
    darwinConfigurations.yourhostname = nix-darwin.lib.darwinSystem {
      modules = [
        opnix.darwinModules.default
        ./configuration.nix
      ];
    };

    # Home Manager
    homeConfigurations.yourusername = home-manager.lib.homeManagerConfiguration {
      modules = [
        opnix.homeManagerModules.default
        ./home.nix
      ];
    };
  };
}
```

Configure secrets:

```nix
# NixOS/nix-darwin
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

# Home Manager
programs.onepassword-secrets = {
  enable = true;
  secrets = {
    sshPrivateKey = {
      reference = "op://Personal/SSH/private-key";
      path = ".ssh/id_rsa";
      mode = "0600";
    };
  };
};
```

Set up your token:

```bash
sudo opnix token set
sudo nixos-rebuild switch --flake .
```

## Documentation

📚 **[Complete Documentation](./docs/README.md)**

- **[Getting Started Guide](./docs/getting-started.md)** - Complete setup walkthrough
- **[Configuration Reference](./docs/configuration-reference.md)** - All configuration options
- **[Examples](./docs/examples/)** - Real-world configuration examples
- **[Best Practices](./docs/best-practices.md)** - Security and operational guidance
- **[Troubleshooting](./docs/troubleshooting.md)** - Common issues and solutions
- **[Migration Guide](./docs/migration-guide.md)** - Upgrading from V0 to V1

## Platform Support

| Platform | Module
 | Use Case |
|----------|--------|----------|
| **NixOS** | `nixosModules.default` | System-wide secret management |
| **nix-darwin** | `darwinModules.default` | macOS system secrets |
| **Home Manager** | `homeManagerModules.default` | User-specific secrets |

## Getting Help

- **📖 Documentation**: Start with the [Getting Started Guide](./docs/getting-started.md)
- **🐛 Issues**: [Report bugs and request features](https://github.com/brizzbuzz/opnix/issues)
- **💬 Discussions**: [Ask questions and share configurations](https://github.com/brizzbuzz/opnix/discussions)

## License

[MIT License](LICENSE)