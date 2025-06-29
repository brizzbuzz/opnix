# Best Practices Guide

This guide covers security, performance, and operational best practices for using OpNix in production environments.

## Security Best Practices

### 1Password Service Account Security

#### Use Dedicated Service Accounts
- **Create separate service accounts** for different environments (dev, staging, prod)
- **Use descriptive names** like `opnix-prod-web-servers` or `opnix-staging-database`
- **Document service account purpose** and scope in your infrastructure documentation

#### Minimal Vault Access
```nix
# Good: Separate vaults for different purposes
secrets = {
  databasePassword = {
    reference = "op://Database-Prod/PostgreSQL/password";
  };
  sslCert = {
    reference = "op://SSL-Certificates/Web-Server/certificate";
  };
};

# Avoid: Using a single vault for everything
# This requires broader service account permissions
```

#### Token Rotation Strategy
- **Rotate tokens quarterly** or after security incidents
- **Use automation** to update tokens across infrastructure
- **Monitor token usage** in 1Password activity logs
- **Have emergency procedures** for token compromise

### Token Management

#### Secure Token Storage
```nix
# Good: Restricted permissions
services.onepassword-secrets = {
  tokenFile = "/etc/opnix-token";  # 640 permissions, root:onepassword-secrets
};

# Avoid: World-readable tokens
# Don't store tokens in /tmp or with 644 permissions
```

#### Token File Best Practices
```bash
# Set up token with proper permissions
sudo opnix token set
sudo chmod 640 /etc/opnix-token
sudo chown root:onepassword-secrets /etc/opnix-token

# Verify permissions
ls -la /etc/opnix-token
# Should show: -rw-r----- 1 root onepassword-secrets
```

#### Environment-Specific Tokens
```nix
# Use different token files for different environments
services.onepassword-secrets = {
  tokenFile = 
    if config.networking.hostName == "prod-server" 
    then "/etc/opnix-prod-token"
    else "/etc/opnix-dev-token";
};
```

### Secret File Security

#### Restrictive Permissions
```nix
# Default to most restrictive permissions
secrets = {
  databasePassword = {
    reference = "op://Vault/DB/password";
    mode = "0600";  # Owner read/write only
    owner = "postgres";
    group = "postgres";
  };
  
  # Only use broader permissions when necessary
  sslCertificate = {
    reference = "op://Vault/SSL/cert";
    mode = "0644";  # Readable by service group
    owner = "caddy";
    group = "caddy";
  };
};
```

#### Dedicated Users and Groups
```nix
# Create dedicated users for services
users.users.app-service = {
  isSystemUser = true;
  group = "app-service";
  home = "/var/lib/app-service";
};

users.groups.app-service = {};

# Use dedicated user for secrets
services.onepassword-secrets.secrets.appConfig = {
  reference = "op://Vault/App/config";
  owner = "app-service";
  group = "app-service";
  mode = "0600";
};
```

#### Secure Secret Paths
```nix
# Good: Use system directories with proper permissions
secrets = {
  sslCert = {
    reference = "op://Vault/SSL/cert";
    path = "/etc/ssl/certs/app.pem";  # Standard system location
  };
};

# Avoid: User-writable or predictable locations
# Don't use /tmp, /var/tmp, or world-writable directories
```

### Network Security

#### Firewall Considerations
- **Ensure outbound HTTPS** (443) is allowed for 1Password API access
- **Monitor network traffic** to 1Password servers
- **Use network policies** to restrict which services can access secrets

#### DNS Security
- **Use secure DNS** resolvers to prevent DNS hijacking
- **Consider DNS over HTTPS** in high-security environments
- **Monitor DNS queries** to 1Password domains

## Performance Best Practices

### Efficient Secret Management

#### Batch Secret Retrieval
```nix
# Good: Group related secrets together
configFiles = [
  ./secrets/database.json     # All database secrets
  ./secrets/web-server.json   # All web server secrets
  ./secrets/monitoring.json   # All monitoring secrets
];

# Avoid: Many small config files
# This creates unnecessary 1Password API calls
```

#### Minimize API Calls
```nix
# Good: Use declarative configuration
secrets = {
  appDatabaseUrl = {
    reference = "op://Vault/App/database-url";
  };
  appApiKey = {
    reference = "op://Vault/App/api-key";
  };
};

# Better: Single config file for related secrets
configFiles = [ ./app-secrets.json ];
```

#### Change Detection Optimization
```nix
services.onepassword-secrets.systemdIntegration = {
  enable = true;
  changeDetection = {
    enable = true;  # Only update when content actually changes
    hashFile = "/var/lib/opnix/secret-hashes";
  };
};
```

### System Resource Management

#### Memory Usage
- **Monitor memory usage** during secret retrieval
- **Use swap if necessary** for systems with limited RAM
- **Consider batch sizes** for large numbers of secrets

#### Disk Space
```nix
# Monitor disk usage in secret directories
services.onepassword-secrets = {
  outputDir = "/var/lib/opnix/secrets";  # Ensure adequate space
};

# Consider log rotation for OpNix logs
services.logrotate.settings.opnix = {
  files = [ "/var/log/opnix*.log" ];
  frequency = "daily";
  rotate = 7;
  compress = true;
};
```

#### Service Startup Optimization
```nix
# Optimize service dependencies
services.onepassword-secrets.systemdIntegration = {
  enable = true;
  services = ["caddy" "postgresql"];  # Only essential services
  restartOnChange = true;
};

# Avoid circular dependencies
# Don't make opnix-secrets depend on services that need secrets
```

## Operational Best Practices

### Configuration Management

#### Version Control
```nix
# Good: Store configuration in version control
services.onepassword-secrets = {
  configFiles = [ ./secrets/config.json ];  # Tracked in git
  secrets = {
    # Declarative configuration tracked in git
  };
};

# Include in your flake.nix inputs
inputs.opnix.url = "github:brizzbuzz/opnix/v0.6.0";  # Pin to specific version
```

#### Configuration Organization
```
secrets/
├── database.json       # Database credentials
├── ssl-certs.json      # SSL certificates
├── api-keys.json       # API keys and tokens
├── monitoring.json     # Monitoring credentials
└── backups.json        # Backup service credentials
```

#### Documentation
```nix
# Document your secrets configuration
services.onepassword-secrets.secrets = {
  databasePassword = {
    reference = "op://Homelab/PostgreSQL-Main/password";
    # Used by: postgresql.service, backup-service
    # Rotation: Monthly via automation
    # Emergency contact: ops-team@company.com
  };
};
```

### Monitoring and Alerting

#### Health Checks
```nix
# Monitor OpNix service health
systemd.services.opnix-health-check = {
  description = "OpNix Health Check";
  serviceConfig = {
    Type = "oneshot";
    ExecStart = pkgs.writeScript "opnix-health-check" ''
      #!/bin/bash
      # Check if secrets are accessible
      if [ ! -r /var/lib/opnix/secrets/database/password ]; then
        echo "ERROR: Database password not accessible"
        exit 1
      fi
      
      # Check token validity
      if ! ${pkgs.opnix}/bin/opnix secret -token-file /etc/opnix-token -validate; then
        echo "ERROR: Token validation failed"
        exit 1
      fi
      
      echo "OpNix health check passed"
    '';
  };
};

systemd.timers.opnix-health-check = {
  wantedBy = [ "timers.target" ];
  timerConfig = {
    OnCalendar = "hourly";
    Persistent = true;
  };
};
```

#### Log Monitoring
```bash
# Monitor OpNix logs for issues
journalctl -u opnix-secrets.service -f

# Set up log alerts for common issues
# - Authentication failures
# - Network connectivity issues
# - Secret retrieval failures
# - Permission denied errors
```

#### Metrics Collection
```nix
# Collect metrics about secret operations
services.prometheus.exporters.node = {
  enable = true;
  enabledCollectors = [ "systemd" ];
};

# Monitor OpNix service metrics:
# - Service status and uptime
# - Secret retrieval duration
# - Error rates
# - Token expiration warnings
```

### Disaster Recovery

#### Backup Strategies
```nix
# Backup strategy for OpNix configuration
services.onepassword-secrets = {
  # Primary configuration
  configFiles = [ ./secrets/prod.json ];
  
  # Emergency fallback configuration
  # configFiles = [ ./secrets/emergency.json ];
};

# Document emergency procedures:
# 1. How to access emergency credentials
# 2. How to switch to backup 1Password account
# 3. How to restore from configuration backups
```

#### Emergency Access
```bash
# Emergency access procedures
# 1. Access to emergency 1Password account
# 2. Local admin access to servers
# 3. Network access during outages
# 4. Contact information for team members

# Emergency token setup
sudo opnix token set -path /etc/opnix-emergency-token
```

#### Recovery Testing
```bash
# Regularly test disaster recovery procedures
# 1. Test with expired tokens
# 2. Test with network outages
# 3. Test with 1Password service outages
# 4. Test configuration rollbacks
```

### Development and Testing

#### Development Environment
```nix
# Separate development configuration
services.onepassword-secrets = {
  enable = lib.mkIf (!config.environment.isDevelopment) true;
  tokenFile = 
    if config.environment.isDevelopment 
    then "/etc/opnix-dev-token"
    else "/etc/opnix-prod-token";
  
  secrets = lib.mkIf config.environment.isDevelopment {
    # Development secrets with fake/test data
    databasePassword = {
      reference = "op://Dev-Vault/Test-DB/password";
    };
  };
};
```

#### Testing Strategy
```bash
# Test configuration changes
nix build .#nixosConfigurations.hostname.config.system.build.toplevel
# Review changes before applying

# Test in staging environment first
nixos-rebuild test --flake .#staging-server

# Validate secrets after deployment
sudo systemctl status opnix-secrets.service
sudo journalctl -u opnix-secrets.service
```

#### CI/CD Integration
```yaml
# GitHub Actions example
name: OpNix Configuration Test
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: cachix/install-nix-action@v22
      - name: Test configuration
        run: |
          nix flake check
          nix build .#nixosConfigurations.test.config.system.build.toplevel
```

## Troubleshooting Best Practices

### Debugging Tools

#### Log Analysis
```bash
# Structured log analysis
journalctl -u opnix-secrets.service --output=json | jq '.MESSAGE'

# Filter for specific issues
journalctl -u opnix-secrets.service | grep -E "(ERROR|WARN)"

# Monitor real-time logs
journalctl -u opnix-secrets.service -f --output=cat
```

#### Configuration Validation
```bash
# Validate configuration before applying
nix-instantiate --eval --strict -E 'with import <nixpkgs> {}; 
  (import ./configuration.nix { inherit pkgs; }).services.onepassword-secrets'

# Check secret references with 1Password CLI
op item get "Database" --vault "Homelab" --format json
```

#### System State Inspection
```bash
# Check secret file permissions
find /var/lib/opnix/secrets -type f -exec ls -la {} \;

# Verify service dependencies
systemctl list-dependencies opnix-secrets.service

# Check group membership
groups $(whoami)
getent group onepassword-secrets
```

### Common Issues and Solutions

#### Permission Problems
```bash
# Fix common permission issues
sudo chown -R root:onepassword-secrets /var/lib/opnix/secrets
sudo chmod -R 640 /var/lib/opnix/secrets
sudo chmod 750 /var/lib/opnix/secrets

# Fix token permissions
sudo chmod 640 /etc/opnix-token
sudo chown root:onepassword-secrets /etc/opnix-token
```

#### Network Issues
```bash
# Test connectivity to 1Password
curl -I https://my.1password.com/api/v1/ping

# Check DNS resolution
nslookup my.1password.com

# Test with 1Password CLI
op account list
op vault list
```

#### Service Integration Issues
```bash
# Check service dependencies
systemctl show opnix-secrets.service -p After
systemctl show caddy.service -p After

# Verify service restart behavior
systemctl restart opnix-secrets.service
systemctl status caddy.service
```

## Migration Best Practices

### Upgrading OpNix Versions

#### Pre-upgrade Checklist
- [ ] **Backup current configuration** and secret files
- [ ] **Review changelog** for breaking changes
- [ ] **Test in staging environment** first
- [ ] **Verify 1Password service account** permissions
- [ ] **Check system compatibility** (NixOS version, etc.)

#### Upgrade Process
```bash
# 1. Update flake input
nix flake update opnix

# 2. Test configuration
nix build .#nixosConfigurations.hostname.config.system.build.toplevel

# 3. Apply in test mode first
sudo nixos-rebuild test --flake .

# 4. Verify services are working
sudo systemctl status opnix-secrets.service

# 5. Apply permanently
sudo nixos-rebuild switch --flake .
```

#### Post-upgrade Validation
```bash
# Verify all secrets are accessible
sudo find /var/lib/opnix/secrets -type f -exec test -r {} \; -print

# Check service integrations
sudo systemctl status caddy.service postgresql.service

# Monitor logs for issues
sudo journalctl -u opnix-secrets.service --since "1 hour ago"
```

### Configuration Migration

#### From V0 to V1
```nix
# V0 Configuration (deprecated)
services.onepassword-secrets = {
  enable = true;
  configFile = ./secrets.json;
  users = ["alice"];
};

# V1 Configuration (recommended)
services.onepassword-secrets = {
  enable = true;
  # Legacy config files still supported
  configFiles = [ ./secrets.json ];
  
  # New declarative format
  secrets = {
    databasePassword = {
      reference = "op://Vault/DB/password";
      owner = "postgres";
      services = ["postgresql"];
    };
  };
};
```

## Security Compliance

### Audit Requirements

#### Access Logging
- **Enable 1Password activity logging** for service accounts
- **Monitor secret access patterns** in application logs
- **Set up alerts** for unusual access patterns
- **Regular access reviews** of service account permissions

#### Compliance Standards
- **SOC 2**: Document secret management procedures
- **ISO 27001**: Include OpNix in information security management
- **PCI DSS**: Ensure proper secret handling for payment data
- **HIPAA**: Secure handling of healthcare-related secrets

#### Documentation Requirements
- **Maintain inventory** of all secrets and their purposes
- **Document access controls** and approval processes
- **Record configuration changes** and their justifications
- **Create incident response procedures** for secret compromise

This best practices guide should be regularly updated as OpNix evolves and new security threats emerge. Always follow your organization's specific security policies and compliance requirements.