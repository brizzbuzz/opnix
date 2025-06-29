# Caddy Web Server with SSL Certificates

This example demonstrates how to use OpNix to manage SSL certificates for a Caddy web server, including automatic service restarts when certificates are updated.

## Prerequisites

- NixOS or nix-darwin system with OpNix configured
- 1Password service account with access to SSL certificates
- SSL certificates stored in 1Password (certificate, private key, and optionally CA bundle)
- Domain name pointing to your server

## 1Password Setup

Create items in your 1Password vault for SSL certificates:

### SSL Certificate Item
```
Vault: Homelab
Item: SSL-Certificate-Example-Com
Fields:
  - certificate: [PEM-encoded certificate]
  - private-key: [PEM-encoded private key]
  - ca-bundle: [PEM-encoded CA bundle] (optional)
  - notes: Issued by Let's Encrypt, expires 2024-03-15
```

### Alternative: Separate Items
You can also store certificate components in separate items:
```
Vault: Homelab
Item: Example-Com-Cert
Fields:
  - certificate: [PEM certificate]

Item: Example-Com-Key  
Fields:
  - private-key: [PEM private key]
```

## Configuration

### Basic Caddy SSL Configuration

```nix
{ config, pkgs, ... }:

{
  # Enable OpNix with SSL certificate management
  services.onepassword-secrets = {
    enable = true;
    tokenFile = "/etc/opnix-token";
    
    secrets = {
      # SSL Certificate
      sslExampleComCert = {
        reference = "op://Homelab/SSL-Certificate-Example-Com/certificate";
        path = "/etc/ssl/certs/example.com.pem";
        owner = "caddy";
        group = "caddy";
        mode = "0644";  # Certificate can be world-readable
        services = ["caddy"];
      };
      
      # SSL Private Key
      sslExampleComKey = {
        reference = "op://Homelab/SSL-Certificate-Example-Com/private-key";
        path = "/etc/ssl/private/example.com.key";
        owner = "caddy";
        group = "caddy";
        mode = "0600";  # Private key must be restricted
        services = ["caddy"];
      };
    };
    
    # Enable advanced systemd integration
    systemdIntegration = {
      enable = true;
      services = ["caddy"];
      restartOnChange = true;
      changeDetection.enable = true;
    };
  };

  # Configure Caddy web server
  services.caddy = {
    enable = true;
    
    # Caddy configuration using OpNix-managed certificates
    virtualHosts."example.com" = {
      extraConfig = ''
        # Use certificates managed by OpNix
        tls ${config.services.onepassword-secrets.secretPaths.sslExampleComCert} ${config.services.onepassword-secrets.secretPaths.sslExampleComKey}
        
        # Your site configuration
        root * /var/www/example.com
        file_server
        
        # Optional: Add security headers
        header {
          # Security headers
          Strict-Transport-Security "max-age=31536000; includeSubDomains; preload"
          X-Content-Type-Options "nosniff"
          X-Frame-Options "DENY"
          X-XSS-Protection "1; mode=block"
          Referrer-Policy "strict-origin-when-cross-origin"
        }
        
        # Optional: Enable gzip compression
        encode gzip
        
        # Optional: Access logging
        log {
          output file /var/log/caddy/example.com.access.log
          format single_field common_log
        }
      '';
    };
  };

  # Create necessary directories
  systemd.tmpfiles.rules = [
    "d /etc/ssl/certs 0755 root root -"
    "d /etc/ssl/private 0700 root root -"
    "d /var/www/example.com 0755 caddy caddy -"
    "d /var/log/caddy 0755 caddy caddy -"
  ];
  
  # Configure firewall
  networking.firewall = {
    allowedTCPPorts = [ 80 443 ];
  };
}
```

### Advanced Configuration with Multiple Sites

```nix
{ config, pkgs, ... }:

{
  services.onepassword-secrets = {
    enable = true;
    tokenFile = "/etc/opnix-token";
    
    # Path template for organized certificate storage
    pathTemplate = "/etc/ssl/{service}/{domain}/{name}";
    defaults = {
      service = "caddy";
    };
    
    secrets = {
      # Main site certificates
      exampleComCert = {
        reference = "op://Homelab/SSL-Example-Com/certificate";
        variables = { domain = "example.com"; };
        owner = "caddy";
        group = "caddy";
        mode = "0644";
        services = ["caddy"];
      };
      
      exampleComKey = {
        reference = "op://Homelab/SSL-Example-Com/private-key";
        variables = { domain = "example.com"; };
        owner = "caddy";
        group = "caddy";
        mode = "0600";
        services = ["caddy"];
      };
      
      # API subdomain certificates
      apiExampleComCert = {
        reference = "op://Homelab/SSL-API-Example-Com/certificate";
        variables = { domain = "api.example.com"; };
        owner = "caddy";
        group = "caddy";
        mode = "0644";
        services = ["caddy"];
      };
      
      apiExampleComKey = {
        reference = "op://Homelab/SSL-API-Example-Com/private-key";
        variables = { domain = "api.example.com"; };
        owner = "caddy";
        group = "caddy";
        mode = "0600";
        services = ["caddy"];
      };
      
      # Wildcard certificate (if available)
      wildcardExampleComCert = {
        reference = "op://Homelab/SSL-Wildcard-Example-Com/certificate";
        path = "/etc/ssl/certs/wildcard.example.com.pem";
        owner = "caddy";
        group = "caddy";
        mode = "0644";
        services = ["caddy"];
      };
      
      wildcardExampleComKey = {
        reference = "op://Homelab/SSL-Wildcard-Example-Com/private-key";
        path = "/etc/ssl/private/wildcard.example.com.key";
        owner = "caddy";
        group = "caddy";
        mode = "0600";
        services = ["caddy"];
      };
    };
    
    systemdIntegration = {
      enable = true;
      services = ["caddy"];
      restartOnChange = true;
      changeDetection = {
        enable = true;
        hashFile = "/var/lib/opnix/ssl-hashes";
      };
      errorHandling = {
        rollbackOnFailure = true;
        continueOnError = false;
      };
    };
  };

  services.caddy = {
    enable = true;
    
    # Main website
    virtualHosts."example.com" = {
      extraConfig = ''
        tls ${config.services.onepassword-secrets.secretPaths.exampleComCert} ${config.services.onepassword-secrets.secretPaths.exampleComKey}
        
        root * /var/www/example.com
        file_server
        
        # Redirect www to non-www
        @www host www.example.com
        redir @www https://example.com{uri} permanent
      '';
    };
    
    # API subdomain
    virtualHosts."api.example.com" = {
      extraConfig = ''
        tls ${config.services.onepassword-secrets.secretPaths.apiExampleComCert} ${config.services.onepassword-secrets.secretPaths.apiExampleComKey}
        
        # Reverse proxy to local API service
        reverse_proxy localhost:8080
        
        # API-specific headers
        header {
          Access-Control-Allow-Origin "*"
          Access-Control-Allow-Methods "GET, POST, PUT, DELETE, OPTIONS"
          Access-Control-Allow-Headers "Content-Type, Authorization"
        }
      '';
    };
    
    # Admin subdomain using wildcard certificate
    virtualHosts."admin.example.com" = {
      extraConfig = ''
        tls ${config.services.onepassword-secrets.secretPaths.wildcardExampleComCert} ${config.services.onepassword-secrets.secretPaths.wildcardExampleComKey}
        
        # Basic authentication for admin area
        basicauth {
          admin $2a$14$Zkx19XLiW6VYouLHR5NmfOFU0z2GTNqnOz4dBN/JdS6DfGjmKb8Sa
        }
        
        root * /var/www/admin
        file_server
      '';
    };
  };

  # Additional security configurations
  systemd.tmpfiles.rules = [
    "d /etc/ssl/caddy 0755 caddy caddy -"
    "d /etc/ssl/caddy/example.com 0755 caddy caddy -"
    "d /etc/ssl/caddy/api.example.com 0755 caddy caddy -"
    "d /var/www/example.com 0755 caddy caddy -"
    "d /var/www/admin 0755 caddy caddy -"
  ];
}
```

### Configuration with Certificate Monitoring

```nix
{ config, pkgs, ... }:

{
  services.onepassword-secrets = {
    enable = true;
    tokenFile = "/etc/opnix-token";
    
    secrets = {
      sslCert = {
        reference = "op://Homelab/SSL-Cert/certificate";
        path = "/etc/ssl/certs/app.pem";
        owner = "caddy";
        group = "caddy";
        mode = "0644";
        services = {
          caddy = {
            restart = true;
            after = ["opnix-secrets.service"];
          };
          # Also notify monitoring service
          cert-monitor = {
            restart = false;
            signal = "SIGHUP";
          };
        };
      };
      
      sslKey = {
        reference = "op://Homelab/SSL-Cert/private-key";
        path = "/etc/ssl/private/app.key";
        owner = "caddy";
        group = "caddy";
        mode = "0600";
        services = ["caddy"];
      };
    };
  };

  services.caddy = {
    enable = true;
    virtualHosts."example.com" = {
      extraConfig = ''
        tls ${config.services.onepassword-secrets.secretPaths.sslCert} ${config.services.onepassword-secrets.secretPaths.sslKey}
        root * /var/www
        file_server
      '';
    };
  };

  # Certificate expiry monitoring service
  systemd.services.cert-monitor = {
    description = "SSL Certificate Expiry Monitor";
    serviceConfig = {
      Type = "notify";
      ExecStart = pkgs.writeScript "cert-monitor" ''
        #!/bin/bash
        
        CERT_FILE="${config.services.onepassword-secrets.secretPaths.sslCert}"
        WARN_DAYS=30
        
        while true; do
          if [ -f "$CERT_FILE" ]; then
            EXPIRY=$(openssl x509 -enddate -noout -in "$CERT_FILE" | cut -d= -f2)
            EXPIRY_EPOCH=$(date -d "$EXPIRY" +%s)
            NOW_EPOCH=$(date +%s)
            DAYS_LEFT=$(( (EXPIRY_EPOCH - NOW_EPOCH) / 86400 ))
            
            echo "Certificate expires in $DAYS_LEFT days"
            
            if [ $DAYS_LEFT -lt $WARN_DAYS ]; then
              echo "WARNING: Certificate expires in $DAYS_LEFT days!"
              # Could send alerts here
            fi
          fi
          
          sleep 3600  # Check every hour
        done
      '';
      Restart = "always";
      RestartSec = "60";
    };
    wantedBy = [ "multi-user.target" ];
    after = [ "opnix-secrets.service" ];
  };
}
```

## Validation

### Check Certificate Deployment

```bash
# Verify certificates are deployed
sudo ls -la /etc/ssl/certs/example.com.pem
sudo ls -la /etc/ssl/private/example.com.key

# Check certificate ownership and permissions
sudo ls -la /etc/ssl/certs/ | grep caddy
sudo ls -la /etc/ssl/private/ | grep caddy

# Verify certificate validity
sudo openssl x509 -in /etc/ssl/certs/example.com.pem -text -noout

# Check certificate expiry
sudo openssl x509 -in /etc/ssl/certs/example.com.pem -enddate -noout
```

### Test Caddy Configuration

```bash
# Check Caddy configuration syntax
sudo caddy validate --config /etc/caddy/Caddyfile

# Test Caddy service
sudo systemctl status caddy.service
sudo journalctl -u caddy.service -f

# Test HTTPS connectivity
curl -I https://example.com
openssl s_client -connect example.com:443 -servername example.com
```

### Verify Service Integration

```bash
# Check service dependencies
systemctl show caddy.service -p After -p Wants

# Test certificate update process
sudo systemctl restart opnix-secrets.service
sudo systemctl status caddy.service

# Monitor logs during certificate updates
sudo journalctl -u opnix-secrets.service -u caddy.service -f
```

## Troubleshooting

### Certificate Not Loading

**Problem**: Caddy reports certificate file not found.

**Solutions**:
1. Check OpNix service status:
   ```bash
   sudo systemctl status opnix-secrets.service
   ```

2. Verify file paths in configuration match actual files:
   ```bash
   sudo ls -la /etc/ssl/certs/
   sudo ls -la /etc/ssl/private/
   ```

3. Check file permissions:
   ```bash
   sudo ls -la /etc/ssl/certs/example.com.pem
   # Should show: -rw-r--r-- 1 caddy caddy
   ```

### Certificate Permission Denied

**Problem**: Caddy cannot read certificate files.

**Solutions**:
1. Fix ownership:
   ```bash
   sudo chown caddy:caddy /etc/ssl/certs/example.com.pem
   sudo chown caddy:caddy /etc/ssl/private/example.com.key
   ```

2. Fix permissions:
   ```bash
   sudo chmod 644 /etc/ssl/certs/example.com.pem
   sudo chmod 600 /etc/ssl/private/example.com.key
   ```

### Service Not Restarting on Certificate Update

**Problem**: Caddy continues using old certificates after OpNix updates them.

**Solutions**:
1. Enable service integration:
   ```nix
   services.onepassword-secrets.secrets.sslCert = {
     services = ["caddy"];  # This should restart Caddy
   };
   ```

2. Check systemd integration:
   ```nix
   services.onepassword-secrets.systemdIntegration = {
     enable = true;
     services = ["caddy"];
     restartOnChange = true;
   };
   ```

### Certificate Chain Issues

**Problem**: Browser shows certificate chain errors.

**Solutions**:
1. Include full certificate chain:
   ```bash
   # Check if certificate includes full chain
   sudo openssl x509 -in /etc/ssl/certs/example.com.pem -text -noout | grep -A5 "Issuer"
   ```

2. Concatenate certificate with CA bundle:
   ```nix
   # Store full chain in 1Password and reference it
   services.onepassword-secrets.secrets.sslFullchain = {
     reference = "op://Homelab/SSL-Cert/fullchain";
     path = "/etc/ssl/certs/example.com-fullchain.pem";
   };
   ```

## Security Considerations

1. **Private Key Security**: Always use mode "0600" for private keys
2. **File Ownership**: Ensure Caddy user owns certificate files
3. **Directory Permissions**: Restrict /etc/ssl/private to mode 700
4. **Certificate Monitoring**: Set up expiry monitoring and alerts
5. **Backup Strategy**: Keep encrypted backups of certificates
6. **Access Logging**: Enable access logs for security monitoring

## Related Examples

- [Nginx SSL Configuration](./nginx-ssl.md) - Alternative web server setup
- [Traefik Configuration](./traefik.md) - Dynamic reverse proxy
- [Multi-Environment Setup](./multi-environment.md) - Managing certificates across environments
- [High Availability](./high-availability.md) - Certificate management in HA setups