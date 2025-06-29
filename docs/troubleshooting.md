# Troubleshooting Guide

This guide helps you diagnose and resolve common issues with OpNix across all supported platforms (NixOS, nix-darwin, and Home Manager).

## Table of Contents

- [General Troubleshooting](#general-troubleshooting)
- [Token and Authentication Issues](#token-and-authentication-issues)
- [Permission and Access Issues](#permission-and-access-issues)
- [Service Integration Issues](#service-integration-issues)
- [Network and Connectivity Issues](#network-and-connectivity-issues)
- [Configuration Issues](#configuration-issues)
- [Platform-Specific Issues](#platform-specific-issues)
- [Performance Issues](#performance-issues)
- [Emergency Procedures](#emergency-procedures)

## General Troubleshooting

### First Steps

When encountering issues with OpNix, always start with these basic checks:

```bash
# 1. Check OpNix service status
sudo systemctl status opnix-secrets.service

# 2. View recent logs
sudo journalctl -u opnix-secrets.service --since "1 hour ago"

# 3. Check configuration syntax
nix-instantiate --eval --strict -E 'with import <nixpkgs> {}; (import ./configuration.nix { inherit pkgs; }).services.onepassword-secrets'

# 4. Verify token file exists and is readable
ls -la /etc/opnix-token
sudo cat /etc/opnix-token | wc -c  # Should be > 0
```

### Common Log Patterns

**Successful Operation:**
```
INFO: Starting OpNix secret deployment
INFO: Processing 3 secrets
INFO: Successfully deployed all secrets
```

**Warning Patterns:**
```
WARNING: Token file /etc/opnix-token does not exist!
INFO: Using existing secrets, skipping updates
```

**Error Patterns:**
```
ERROR: Authentication failed
ERROR: Cannot write secret file
ERROR: 1Password reference not found
```

### Quick Fixes Checklist

- [ ] Token file exists and has correct permissions
- [ ] Network connectivity to 1Password servers
- [ ] 1Password service account has vault access
- [ ] Secret references are correctly formatted
- [ ] Target directories exist and are writable
- [ ] Services have correct user/group ownership

## Token and Authentication Issues

### Issue: Token File Not Found

**Symptoms:**
```
WARNING: Token file /etc/opnix-token does not exist!
INFO: Using existing secrets, skipping updates
INFO: Run 'opnix token set' to configure the token
```

**Solutions:**

1. **Set up the token:**
   ```bash
   sudo opnix token set
   ```

2. **Verify token file location:**
   ```bash
   ls -la /etc/opnix-token
   # Should show: -rw-r----- 1 root onepassword-secrets
   ```

3. **Check configuration for custom token path:**
   ```nix
   services.onepassword-secrets = {
     tokenFile = "/custom/path/to/token";  # Check this path
   };
   ```

### Issue: Authentication Failed

**Symptoms:**
```
ERROR: Authentication failed
INFO: Token may be expired or invalid
ERROR: failed to authenticate with 1Password
```

**Diagnosis:**
```bash
# Test token manually with 1Password CLI
export OP_SERVICE_ACCOUNT_TOKEN="$(sudo cat /etc/opnix-token)"
op account list
op vault list
```

**Solutions:**

1. **Regenerate service account token:**
   - Go to 1Password Developer Console
   - Regenerate token for your service account
   - Update token: `sudo opnix token set`

2. **Verify service account permissions:**
   - Check vault access in 1Password console
   - Ensure service account has read access to required vaults

3. **Check token format:**
   ```bash
   # Token should be a single line without extra whitespace
   sudo cat /etc/opnix-token | hexdump -C | head -5
   ```

### Issue: Token Permissions

**Symptoms:**
```
ERROR: Token file /etc/opnix-token is not readable!
INFO: Check file permissions or group membership
```

**Solutions:**

1. **Fix token permissions:**
   ```bash
   sudo chmod 640 /etc/opnix-token
   sudo chown root:onepassword-secrets /etc/opnix-token
   ```

2. **Check group membership:**
   ```bash
   # Verify group exists
   getent group onepassword-secrets
   
   # Check user membership (for Home Manager)
   groups $(whoami)
   ```

3. **Add user to group (if needed):**
   ```bash
   sudo usermod -a -G onepassword-secrets $(whoami)
   # Logout and login again for group changes to take effect
   ```

## Permission and Access Issues

### Issue: Cannot Write Secret File

**Symptoms:**
```
ERROR: Cannot write secret file
Secret: ssl/cert
Target: /etc/ssl/certs/app.pem
Issue: Permission denied - OpNix cannot write to /etc/ssl/certs/
```

**Solutions:**

1. **Create parent directories:**
   ```bash
   sudo mkdir -p /etc/ssl/certs
   sudo chmod 755 /etc/ssl/certs
   ```

2. **Check directory ownership:**
   ```bash
   ls -la /etc/ssl/
   # Ensure parent directory is writable by root
   ```

3. **Verify OpNix service user:**
   ```bash
   systemctl show opnix-secrets.service -p User -p Group
   ```

### Issue: Secret File Not Accessible to Service

**Symptoms:**
```
# Service logs show permission denied when accessing secrets
ERROR: Could not read certificate file /var/lib/opnix/secrets/ssl/cert.pem
```

**Diagnosis:**
```bash
# Check secret file permissions
ls -la /var/lib/opnix/secrets/ssl/cert.pem

# Check service user
systemctl show caddy.service -p User -p Group

# Test access as service user
sudo -u caddy cat /var/lib/opnix/secrets/ssl/cert.pem
```

**Solutions:**

1. **Fix secret ownership:**
   ```nix
   services.onepassword-secrets.secrets.sslCert = {
     reference = "op://Vault/SSL/cert";
     owner = "caddy";
     group = "caddy";
     mode = "0644";
   };
   ```

2. **Use group access:**
   ```nix
   services.onepassword-secrets.secrets.sslCert = {
     reference = "op://Vault/SSL/cert";
     owner = "root";
     group = "ssl-cert";
     mode = "0640";
   };
   
   # Add service user to group
   users.users.caddy.extraGroups = [ "ssl-cert" ];
   ```

### Issue: Home Manager Permission Problems

**Symptoms:**
```
ERROR: Cannot access system token at /etc/opnix-token
INFO: Make sure the system token can be accessed by your user
```

**Solutions:**

1. **Add user to onepassword-secrets group:**
   ```bash
   sudo usermod -a -G onepassword-secrets $(whoami)
   newgrp onepassword-secrets  # Activate group immediately
   ```

2. **Use user-specific token:**
   ```nix
   programs.onepassword-secrets = {
     tokenFile = "${config.home.homeDirectory}/.config/opnix/token";
   };
   ```

3. **Set up user token:**
   ```bash
   mkdir -p ~/.config/opnix
   opnix token set -path ~/.config/opnix/token
   chmod 600 ~/.config/opnix/token
   ```

## Service Integration Issues

### Issue: Services Don't Wait for Secrets

**Symptoms:**
```
# Service fails to start because secrets aren't available yet
ERROR: Certificate file not found: /var/lib/opnix/secrets/ssl/cert.pem
```

**Solutions:**

1. **Enable systemd integration:**
   ```nix
   services.onepassword-secrets.systemdIntegration = {
     enable = true;
     services = ["caddy" "postgresql"];
   };
   ```

2. **Manual service dependencies:**
   ```nix
   systemd.services.caddy = {
     after = [ "opnix-secrets.service" ];
     wants = [ "opnix-secrets.service" ];
   };
   ```

3. **Verify dependencies were applied:**
   ```bash
   systemctl show caddy.service -p After -p Wants
   # Should include opnix-secrets.service
   ```

### Issue: Services Don't Restart on Secret Changes

**Symptoms:**
Services continue using old secrets after OpNix updates them.

**Solutions:**

1. **Enable restart on change:**
   ```nix
   services.onepassword-secrets.secrets.sslCert = {
     reference = "op://Vault/SSL/cert";
     services = ["caddy"];  # Will restart caddy when cert changes
   };
   ```

2. **Advanced service control:**
   ```nix
   services.onepassword-secrets.secrets.configFile = {
     reference = "op://Vault/Config/file";
     services = {
       myservice = {
         restart = false;
         signal = "SIGHUP";  # Send reload signal instead
       };
     };
   };
   ```

3. **Enable change detection:**
   ```nix
   services.onepassword-secrets.systemdIntegration = {
     enable = true;
     changeDetection.enable = true;
     restartOnChange = true;
   };
   ```

### Issue: Circular Dependencies

**Symptoms:**
```
ERROR: Found dependency loop involving opnix-secrets.service
```

**Solutions:**

1. **Review service dependencies:**
   ```bash
   systemctl list-dependencies opnix-secrets.service
   systemctl list-dependencies --reverse opnix-secrets.service
   ```

2. **Avoid making OpNix depend on services that need secrets:**
   ```nix
   # Don't do this:
   systemd.services.opnix-secrets = {
     after = [ "postgresql.service" ];  # Creates circular dependency
   };
   ```

3. **Use proper service ordering:**
   ```nix
   # Instead, make services depend on OpNix:
   services.onepassword-secrets.systemdIntegration = {
     enable = true;
     services = ["postgresql"];
   };
   ```

## Network and Connectivity Issues

### Issue: Cannot Connect to 1Password

**Symptoms:**
```
ERROR: failed to connect to 1Password servers
ERROR: network unreachable
```

**Diagnosis:**
```bash
# Test basic connectivity
curl -I https://my.1password.com/api/v1/ping

# Check DNS resolution
nslookup my.1password.com

# Test with 1Password CLI
op account list
```

**Solutions:**

1. **Check firewall rules:**
   ```bash
   # Ensure outbound HTTPS (443) is allowed
   sudo ufw status
   sudo iptables -L OUTPUT
   ```

2. **Check proxy settings:**
   ```bash
   echo $https_proxy
   echo $HTTPS_PROXY
   
   # If using proxy, configure for OpNix service
   systemd.services.opnix-secrets.environment = {
     https_proxy = "http://proxy.example.com:8080";
   };
   ```

3. **DNS issues:**
   ```bash
   # Check DNS configuration
   cat /etc/resolv.conf
   
   # Test with different DNS
   nslookup my.1password.com 8.8.8.8
   ```

### Issue: Timeout Connecting to 1Password

**Symptoms:**
```
ERROR: timeout connecting to 1Password API
ERROR: request timeout after 30 seconds
```

**Solutions:**

1. **Check network latency:**
   ```bash
   ping my.1password.com
   traceroute my.1password.com
   ```

2. **Increase timeout (if supported):**
   ```bash
   # Check if timeout options are available
   opnix secret --help | grep -i timeout
   ```

3. **Retry mechanism:**
   ```nix
   services.onepassword-secrets.systemdIntegration = {
     errorHandling = {
       maxRetries = 5;
       continueOnError = true;
     };
   };
   ```

## Configuration Issues

### Issue: Invalid 1Password Reference

**Symptoms:**
```
ERROR: 1Password reference not found
Secret: api/key
Reference: op://Vault/Missing-Item/field
Issue: Item 'Missing-Item' not found in vault 'Vault'
```

**Diagnosis:**
```bash
# Test reference with 1Password CLI
export OP_SERVICE_ACCOUNT_TOKEN="$(sudo cat /etc/opnix-token)"
op item get "Missing-Item" --vault "Vault"

# List available items
op item list --vault "Vault"
```

**Solutions:**

1. **Verify item exists:**
   ```bash
   op item list --vault "Vault" | grep -i "missing"
   ```

2. **Check exact item name:**
   ```bash
   # Item names are case-sensitive
   op item get "missing-item" --vault "Vault"  # Try lowercase
   op item get "Missing Item" --vault "Vault"  # Try with spaces
   ```

3. **Verify vault access:**
   ```bash
   # List accessible vaults
   op vault list
   
   # Check service account permissions in 1Password console
   ```

### Issue: Configuration Validation Errors

**Symptoms:**
```
ERROR: Invalid configuration
Secret: database/password
Issue: User 'nonexistent-user' does not exist
```

**Solutions:**

1. **Create missing users:**
   ```nix
   users.users.nonexistent-user = {
     isSystemUser = true;
     group = "nonexistent-user";
   };
   users.groups.nonexistent-user = {};
   ```

2. **Fix configuration:**
   ```nix
   services.onepassword-secrets.secrets.databasePassword = {
     reference = "op://Vault/DB/password";
     owner = "postgres";  # Use existing user
     group = "postgres";
   };
   ```

3. **Validate before applying:**
   ```bash
   # Check user exists
   getent passwd postgres
   getent group postgres
   ```

### Issue: Path Conflicts

**Symptoms:**
```
ERROR: Multiple secrets configured for the same path
Path: /etc/ssl/certs/app.pem
Secrets: ssl/cert, ssl/certificate
```

**Solutions:**

1. **Use different paths:**
   ```nix
   services.onepassword-secrets.secrets = {
     sslCert = {
       reference = "op://Vault/SSL/cert";
       path = "/etc/ssl/certs/app-cert.pem";
     };
     sslCertificate = {
       reference = "op://Vault/SSL/fullchain";
       path = "/etc/ssl/certs/app-fullchain.pem";
     };
   };
   ```

2. **Use symlinks:**
   ```nix
   services.onepassword-secrets.secrets.sslCert = {
     reference = "op://Vault/SSL/cert";
     path = "/etc/ssl/certs/app.pem";
     symlinks = [
       "/etc/ssl/certs/legacy.pem"
       "/opt/service/ssl/cert.pem"
     ];
   };
   ```

## Platform-Specific Issues

### NixOS Issues

#### Issue: systemd Service Not Starting

**Symptoms:**
```
● opnix-secrets.service - OpNix Secret Management
   Loaded: loaded
   Active: failed (Result: exit-code)
```

**Solutions:**

1. **Check service logs:**
   ```bash
   journalctl -u opnix-secrets.service -f
   ```

2. **Check service definition:**
   ```bash
   systemctl cat opnix-secrets.service
   ```

3. **Manual service test:**
   ```bash
   sudo systemctl start opnix-secrets.service
   sudo systemctl status opnix-secrets.service
   ```

### nix-darwin Issues

#### Issue: launchd Service Problems

**Symptoms:**
```
# Service not running on macOS
sudo launchctl list | grep opnix
# No output
```

**Solutions:**

1. **Check launchd service:**
   ```bash
   sudo launchctl list org.nixos.opnix-secrets
   sudo launchctl print system/org.nixos.opnix-secrets
   ```

2. **Check service logs:**
   ```bash
   tail -f /var/log/opnix-secrets.log
   ```

3. **Restart service:**
   ```bash
   sudo launchctl unload /Library/LaunchDaemons/org.nixos.opnix-secrets.plist
   sudo launchctl load /Library/LaunchDaemons/org.nixos.opnix-secrets.plist
   ```

#### Issue: Group Permission Problems

**Symptoms:**
```
ERROR: User not in onepassword-secrets group
```

**Solutions:**

1. **Check group exists:**
   ```bash
   dscl . read /Groups/onepassword-secrets
   ```

2. **Verify group ID:**
   ```bash
   dscl . list /Groups PrimaryGroupID | grep onepassword-secrets
   ```

3. **Fix group configuration:**
   ```nix
   services.onepassword-secrets = {
     groupId = 601;  # Use different unused GID
     users = ["yourusername"];
   };
   ```

### Home Manager Issues

#### Issue: Secrets Not Deployed to Home Directory

**Symptoms:**
Home Manager activation completes but secrets aren't in expected locations.

**Solutions:**

1. **Check Home Manager logs:**
   ```bash
   journalctl --user -u home-manager-yourusername.service
   ```

2. **Verify configuration:**
   ```nix
   programs.onepassword-secrets.secrets.sshKey = {
     reference = "op://Personal/SSH/key";
     path = ".ssh/id_rsa";  # Relative to home directory
   };
   ```

3. **Check activation output:**
   ```bash
   home-manager switch --verbose
   ```

## Performance Issues

### Issue: Slow Secret Retrieval

**Symptoms:**
OpNix takes a long time to retrieve secrets during system activation.

**Solutions:**

1. **Enable change detection:**
   ```nix
   services.onepassword-secrets.systemdIntegration = {
     changeDetection.enable = true;
   };
   ```

2. **Optimize configuration:**
   ```nix
   # Group related secrets in single config files
   services.onepassword-secrets = {
     configFiles = [ ./all-secrets.json ];  # Better than many small files
   };
   ```

3. **Monitor performance:**
   ```bash
   # Time secret retrieval
   time sudo systemctl restart opnix-secrets.service
   
   # Check service timing
   systemctl show opnix-secrets.service -p ExecMainStartTimestamp -p ExecMainExitTimestamp
   ```

### Issue: High Memory Usage

**Solutions:**

1. **Monitor memory usage:**
   ```bash
   systemctl show opnix-secrets.service -p MemoryCurrent
   ```

2. **Limit memory if needed:**
   ```nix
   systemd.services.opnix-secrets = {
     serviceConfig = {
       MemoryMax = "100M";
     };
   };
   ```

## Emergency Procedures

### Emergency: System Won't Boot

**OpNix V1 should never cause boot failures**, but if you suspect it's related:

1. **Boot from rescue media** or previous generation:
   ```bash
   # At GRUB menu, select previous generation
   # Or boot from NixOS installation media
   ```

2. **Disable OpNix temporarily:**
   ```bash
   # Mount your system
   mount /dev/sdaX /mnt
   
   # Edit configuration to disable OpNix
   nano /mnt/etc/nixos/configuration.nix
   # Comment out or set: services.onepassword-secrets.enable = false;
   
   # Rebuild
   nixos-rebuild switch --root /mnt
   ```

3. **Fix configuration and re-enable:**
   ```bash
   # After fixing the issue, re-enable OpNix
   services.onepassword-secrets.enable = true;
   ```

### Emergency: All Secrets Inaccessible

1. **Check if files exist:**
   ```bash
   ls -la /var/lib/opnix/secrets/
   ```

2. **Restore from backup:**
   ```bash
   # If you have backups
   sudo cp -r /backup/opnix/secrets/* /var/lib/opnix/secrets/
   sudo chown -R root:onepassword-secrets /var/lib/opnix/secrets/
   sudo chmod -R 640 /var/lib/opnix/secrets/
   ```

3. **Force secret retrieval:**
   ```bash
   # Remove hash file to force refresh
   sudo rm -f /var/lib/opnix/secret-hashes
   sudo systemctl restart opnix-secrets.service
   ```

### Emergency: Token Compromised

1. **Immediate actions:**
   ```bash
   # Remove token file
   sudo rm /etc/opnix-token
   
   # Restart OpNix service (will use existing secrets)
   sudo systemctl restart opnix-secrets.service
   ```

2. **Regenerate token in 1Password console**

3. **Update token:**
   ```bash
   sudo opnix token set
   ```

## Getting Additional Help

### Gathering Debug Information

When reporting issues, include:

```bash
#!/bin/bash
# OpNix Debug Information Collection Script

echo "=== OpNix Debug Information ==="
echo "Date: $(date)"
echo "Hostname: $(hostname)"
echo "OS: $(uname -a)"
echo

echo "=== OpNix Service Status ==="
systemctl status opnix-secrets.service
echo

echo "=== Recent Logs ==="
journalctl -u opnix-secrets.service --since "1 hour ago" --no-pager
echo

echo "=== Configuration ==="
nix-instantiate --eval --strict -E 'with import <nixpkgs> {}; (import ./configuration.nix { inherit pkgs; }).services.onepassword-secrets' 2>/dev/null || echo "Could not evaluate configuration"
echo

echo "=== Token File ==="
ls -la /etc/opnix-token 2>/dev/null || echo "Token file not found"
echo

echo "=== Secret Files ==="
if [ -d /var/lib/opnix/secrets ]; then
  find /var/lib/opnix/secrets -type f -exec ls -la {} \;
else
  echo "Secret directory not found"
fi
echo

echo "=== Network Connectivity ==="
curl -I https://my.1password.com/api/v1/ping 2>&1 | head -5
echo

echo "=== 1Password CLI Test ==="
if command -v op >/dev/null; then
  op account list 2>&1 | head -5
else
  echo "1Password CLI not available"
fi
echo

echo "=== End Debug Information ==="
```

### Community Support

- **GitHub Issues**: [Report bugs and ask questions](https://github.com/brizzbuzz/opnix/issues)
- **Documentation**: Check the full documentation in the `docs/` directory
- **Examples**: Review configuration examples for similar setups

### Professional Support

For production environments requiring guaranteed support:
- Consider commercial 1Password support
- Engage with NixOS professional services
- Consider infrastructure consulting services

Remember to never share actual tokens, secrets, or sensitive configuration details when seeking help. Always sanitize debug information before sharing publicly.