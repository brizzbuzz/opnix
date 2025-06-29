# OpNix Examples

This directory contains real-world configuration examples for common OpNix use cases across different platforms and scenarios.

## Table of Contents

### Basic Examples
- [Basic NixOS Setup](./basic-nixos.md) - Simple systemd-based secret management
- [Basic nix-darwin Setup](./basic-darwin.md) - macOS system secret management
- [Basic Home Manager Setup](./basic-home-manager.md) - User-level secret management

### Web Services
- [Caddy Web Server](./caddy-ssl.md) - SSL certificates and reverse proxy configuration
- [Nginx with SSL](./nginx-ssl.md) - Web server with automatic SSL certificate management
- [Traefik Reverse Proxy](./traefik.md) - Dynamic reverse proxy with secret management

### Databases
- [PostgreSQL](./postgresql.md) - Database credentials and SSL certificates
- [MySQL/MariaDB](./mysql.md) - Database setup with user credentials
- [Redis](./redis.md) - Redis authentication and SSL configuration

### Application Services
- [Grafana](./grafana.md) - Monitoring dashboards with secret keys
- [Nextcloud](./nextcloud.md) - Self-hosted cloud with database and admin credentials
- [GitLab Runner](./gitlab-runner.md) - CI/CD runner with registration tokens

### Advanced Patterns
- [Multi-Environment Setup](./multi-environment.md) - Dev/staging/prod secret management
- [Service Orchestration](./service-orchestration.md) - Complex service dependencies
- [Path Templates](./path-templates.md) - Organized secret paths with variables
- [High Availability](./high-availability.md) - Secrets in HA cluster setups

### Platform-Specific
- [NixOS Server](./nixos-server.md) - Complete server configuration with secrets
- [macOS Development](./macos-development.md) - Development environment secrets
- [Home Manager Multi-User](./home-manager-multi-user.md) - Shared system with per-user secrets

### Integration Examples
- [Docker Compose](./docker-compose.md) - Container secrets via OpNix
- [Kubernetes](./kubernetes.md) - Kubernetes secret management integration
- [Terraform](./terraform.md) - Infrastructure automation with secret management

### Migration Examples
- [V0 to V1 Migration](./v0-to-v1-migration.md) - Step-by-step migration examples
- [Legacy System Integration](./legacy-integration.md) - Integrating with existing systems

## Quick Reference

### Common Patterns

#### Basic Secret Declaration
```nix
services.onepassword-secrets.secrets = {
  serviceCredential = {
    reference = "op://Vault/Item/field";
    owner = "service-user";
    group = "service-group";
    mode = "0600";
    services = ["service-name"];
  };
  
  sslCertificate = {
    reference = "op://Homelab/SSL Certificates/example.com/cert";
    owner = "caddy";
    group = "caddy";
    mode = "0644";
  };
};
```

#### Custom Path with Service Integration
```nix
services.onepassword-secrets.secrets = {
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
```

#### Home Manager User Secrets
```nix
programs.onepassword-secrets.secrets = {
  sshPrivateKey = {
    reference = "op://Personal/SSH/private-key";
    path = ".ssh/id_rsa";
    mode = "0600";
  };
};
```

### Configuration File Examples

#### Simple JSON Configuration
```json
{
  "secrets": [
    {
      "path": "databasePassword",
      "reference": "op://Homelab/Database/password"
    }
  ]
}
```

#### Multi-Service JSON Configuration
```json
{
  "secrets": [
    {
      "path": "caddy/ssl-cert",
      "reference": "op://Homelab/SSL/certificate",
      "owner": "caddy",
      "group": "caddy",
      "mode": "0644"
    },
    {
      "path": "postgres/password",
      "reference": "op://Homelab/Database/password",
      "owner": "postgres",
      "group": "postgres",
      "mode": "0600"
    }
  ]
}
```

## Getting Started

1. **Choose an example** that matches your use case
2. **Review the prerequisites** for each example
3. **Adapt the configuration** to your specific environment
4. **Test in a development environment** first
5. **Apply to production** after validation

## Contributing Examples

If you have a working OpNix configuration that others might find useful:

1. **Create a new markdown file** following the naming convention
2. **Include complete configuration** with explanations
3. **Add prerequisites and setup steps**
4. **Include troubleshooting tips** specific to your use case
5. **Submit a pull request** with your example

### Example Template

```markdown
# Service Name Configuration

Brief description of what this example demonstrates.

## Prerequisites

- List of requirements
- Specific versions if needed
- External dependencies

## Configuration

### 1Password Setup
Instructions for setting up items in 1Password.

### NixOS Configuration
Complete Nix configuration with explanations.

### Validation
Steps to verify the configuration works.

## Troubleshooting

Common issues and solutions specific to this example.

## Related Examples

Links to related configurations.
```

## Best Practices Demonstrated

Each example demonstrates one or more best practices:

- **Security**: Proper permissions and ownership
- **Reliability**: Service dependencies and error handling
- **Maintainability**: Clear configuration structure
- **Performance**: Efficient secret management
- **Monitoring**: Health checks and logging

## Platform Support Matrix

| Example | NixOS | nix-darwin | Home Manager |
|---------|-------|------------|--------------|
| Basic Setup | ✅ | ✅ | ✅ |
| Web Services | ✅ | ✅ | ❌ |
| Databases | ✅ | ✅ | ❌ |
| Development | ✅ | ✅ | ✅ |
| Multi-User | ✅ | ✅ | ✅ |

Legend:
- ✅ Supported and documented
- ❌ Not applicable for this platform
- ⚠️ Partial support or platform-specific limitations

## Need Help?

- **Check the [Troubleshooting Guide](../troubleshooting.md)** for common issues
- **Review [Best Practices](../best-practices.md)** for security recommendations
- **Read the [Configuration Reference](../configuration-reference.md)** for detailed options
- **Open an issue** on GitHub if you can't find what you need

Remember to never commit actual secrets or tokens to version control when adapting these examples!