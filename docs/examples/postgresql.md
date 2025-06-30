# PostgreSQL Database with OpNix Secret Management

This example demonstrates how to use OpNix to manage PostgreSQL database credentials, SSL certificates, and configuration secrets with automatic service integration.

## Prerequisites

- NixOS or nix-darwin system with OpNix configured
- 1Password service account with access to database credentials
- PostgreSQL database credentials stored in 1Password
- Optional: SSL certificates for PostgreSQL connections

## 1Password Setup

Create items in your 1Password vault for PostgreSQL credentials:

### Database Credentials Item
```
Vault: Homelab
Item: PostgreSQL-Main-Database
Fields:
  - username: postgres
  - password: [secure database password]
  - database: myapp_production
  - host: localhost
  - port: 5432
  - connection-string: postgresql://postgres:[password]@localhost:5432/myapp_production
  - notes: Main application database
```

### Database User Credentials
```
Vault: Homelab
Item: PostgreSQL-App-User
Fields:
  - username: myapp_user
  - password: [secure user password]
  - database: myapp_production
  - privileges: SELECT, INSERT, UPDATE, DELETE
```

### SSL Certificate (Optional)
```
Vault: Homelab
Item: PostgreSQL-SSL-Certificate
Fields:
  - server-cert: [PEM-encoded server certificate]
  - server-key: [PEM-encoded private key]
  - ca-cert: [PEM-encoded CA certificate]
```

## Configuration

### Basic PostgreSQL Setup

```nix
{ config, pkgs, ... }:

{
  # Enable OpNix with PostgreSQL secret management
  services.onepassword-secrets = {
    enable = true;
    tokenFile = "/etc/opnix-token";
    
    secrets = {
      # PostgreSQL superuser password
      postgresSuperuserPassword = {
        reference = "op://Homelab/PostgreSQL-Main/superuser-password";
        owner = "postgres";
        group = "postgres";
        mode = "0600";
        services = ["postgresql"];
      };
      
      # Application user password
      postgresAppUserPassword = {
        reference = "op://Homelab/PostgreSQL-Main/app-user-password";
        owner = "postgres";
        group = "postgres";
        mode = "0600";
        services = ["postgresql"];
      };
      
      # Database connection string for applications
      postgresConnectionString = {
        reference = "op://Homelab/PostgreSQL-Main/connection-string";
        path = "/etc/postgresql/connection-string";
        owner = "myapp";
        group = "myapp";
        mode = "0600";
        services = ["postgresql"];
      };
    };
    
    # Enable systemd integration
    systemdIntegration = {
      enable = true;
      services = ["postgresql" "myapp"];
      restartOnChange = true;
      changeDetection.enable = true;
    };
  };

  # Configure PostgreSQL service
  services.postgresql = {
    enable = true;
    package = pkgs.postgresql_15;
    
    # Use OpNix-managed password for postgres user
    initialScript = pkgs.writeText "postgres-init.sql" ''
      -- Set postgres user password from OpNix secret
      ALTER USER postgres PASSWORD '$(cat ${config.services.onepassword-secrets.secretPaths."postgres/superuser-password"})';
      
      -- Create application database
      CREATE DATABASE myapp_production;
      
      -- Create application user with password from OpNix secret
      CREATE USER myapp_user WITH PASSWORD '$(cat ${config.services.onepassword-secrets.secretPaths."postgres/app-user-password"})';
      
      -- Grant privileges to application user
      GRANT ALL PRIVILEGES ON DATABASE myapp_production TO myapp_user;
      
      -- Connect to the application database
      \c myapp_production
      
      -- Grant schema privileges
      GRANT ALL ON SCHEMA public TO myapp_user;
      GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO myapp_user;
      GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO myapp_user;
      
      -- Set default privileges for future objects
      ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO myapp_user;
      ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO myapp_user;
    '';
    
    # Authentication configuration
    authentication = pkgs.lib.mkOverride 10 ''
      # Local connections
      local   all             all                                     peer
      
      # IPv4 local connections with password
      host    all             all             127.0.0.1/32            md5
      host    all             all             ::1/128                 md5
      
      # Application connections
      host    myapp_production myapp_user      127.0.0.1/32            md5
      host    myapp_production myapp_user      ::1/128                 md5
    '';
    
    # PostgreSQL settings
    settings = {
      # Performance settings
      shared_buffers = "256MB";
      effective_cache_size = "1GB";
      maintenance_work_mem = "64MB";
      checkpoint_completion_target = 0.9;
      wal_buffers = "16MB";
      default_statistics_target = 100;
      random_page_cost = 1.1;
      effective_io_concurrency = 200;
      
      # Logging settings
      log_destination = "stderr";
      logging_collector = true;
      log_directory = "/var/log/postgresql";
      log_filename = "postgresql-%Y-%m-%d_%H%M%S.log";
      log_statement = "mod";
      log_min_duration_statement = 1000;
      
      # Connection settings
      max_connections = 100;
      port = 5432;
    };
    
    # Enable JIT for better performance (PostgreSQL 11+)
    enableJIT = true;
  };

  # Create application user
  users.users.myapp = {
    isSystemUser = true;
    group = "myapp";
    home = "/var/lib/myapp";
    createHome = true;
  };
  
  users.groups.myapp = {};

  # Create necessary directories
  systemd.tmpfiles.rules = [
    "d /var/log/postgresql 0755 postgres postgres -"
    "d /run/secrets 0755 root root -"
  ];
}
```

### PostgreSQL with SSL Configuration

```nix
{ config, pkgs, ... }:

{
  services.onepassword-secrets = {
    enable = true;
    tokenFile = "/etc/opnix-token";
    
    secrets = {
      # PostgreSQL superuser password
      postgresSuperuserPassword = {
        reference = "op://Homelab/PostgreSQL-Main/superuser-password";
        owner = "postgres";
        group = "postgres";
        mode = "0600";
        services = ["postgresql"];
      };
      
      # SSL certificate for PostgreSQL
      postgresSslCert = {
        reference = "op://Homelab/PostgreSQL-SSL/certificate";
        path = "/var/lib/postgresql/server.crt";
        owner = "postgres";
        group = "postgres";
        mode = "0600";
        services = ["postgresql"];
      };
      
      # SSL private key for PostgreSQL
      postgresSslKey = {
        reference = "op://Homelab/PostgreSQL-SSL/private-key";
        path = "/var/lib/postgresql/server.key";
        owner = "postgres";
        group = "postgres";
        mode = "0600";
        services = ["postgresql"];
      };
      
      # SSL CA certificate
      postgresCaCert = {
        reference = "op://Homelab/PostgreSQL-SSL/ca-certificate";
        path = "/var/lib/postgresql/ca.crt";
        owner = "postgres";
        group = "postgres";
        mode = "0644";
        services = ["postgresql"];
      };
    };
  };

  services.postgresql = {
    enable = true;
    package = pkgs.postgresql_15;
    
    # SSL-enabled authentication
    authentication = ''
      # SSL connections
      hostssl all             all             0.0.0.0/0               md5
      hostssl all             all             ::/0                    md5
      
      # Local connections
      local   all             all                                     peer
    '';
    
    settings = {
      # SSL Configuration
      ssl = true;
      ssl_cert_file = config.services.onepassword-secrets.secretPaths."postgres/ssl-cert";
      ssl_key_file = config.services.onepassword-secrets.secretPaths."postgres/ssl-key";
      ssl_ca_file = config.services.onepassword-secrets.secretPaths."postgres/ca-cert";
      ssl_ciphers = "HIGH:MEDIUM:+3DES:!aNULL";
      ssl_prefer_server_ciphers = true;
      
      # Network settings
      listen_addresses = "*";
      port = 5432;
      max_connections = 100;
      
      # Security settings
      password_encryption = "scram-sha-256";
      
      # Performance settings
      shared_buffers = "256MB";
      effective_cache_size = "1GB";
    };
  };

  # Firewall configuration for SSL connections
  networking.firewall = {
    allowedTCPPorts = [ 5432 ];
  };
}
```

### Advanced Multi-Database Setup

```nix
{ config, pkgs, lib, ... }:

let
  databases = {
    app_production = {
      user = "app_user";
      passwordRef = "op://Homelab/App-Production-DB/password";
    };
    analytics = {
      user = "analytics_user";
      passwordRef = "op://Homelab/Analytics-DB/password";
    };
    sessions = {
      user = "sessions_user";
      passwordRef = "op://Homelab/Sessions-DB/password";
    };
  };
in {
  services.onepassword-secrets = {
    enable = true;
    tokenFile = "/etc/opnix-token";
    
    # Path template for organized secrets
    pathTemplate = "/run/secrets/postgres/{database}/{name}";
    
    secrets = 
      # PostgreSQL superuser password
      {
        "postgres-superuser-password" = {
          reference = "op://Homelab/PostgreSQL-Main/password";
          path = "/run/secrets/postgres/superuser-password";
          owner = "postgres";
          group = "postgres";
          mode = "0600";
          services = ["postgresql"];
        };
      } //
      # Generate secrets for each database
      (lib.mapAttrs' (dbName: dbConfig: 
        lib.nameValuePair "db-${dbName}-password" {
          reference = dbConfig.passwordRef;
          variables = { database = dbName; name = "password"; };
          owner = "postgres";
          group = "postgres";
          mode = "0600";
          services = ["postgresql"];
        }
      ) databases);
  };

  services.postgresql = {
    enable = true;
    package = pkgs.postgresql_15;
    
    # Create initialization script for all databases
    initialScript = pkgs.writeText "postgres-multi-init.sql" ''
      -- Set postgres superuser password
      ALTER USER postgres PASSWORD '$(cat ${config.services.onepassword-secrets.secretPaths."postgres-superuser-password"})';
      
      ${lib.concatMapStringsSep "\n" (dbName: let
        dbConfig = databases.${dbName};
        passwordPath = config.services.onepassword-secrets.secretPaths."db-${dbName}-password";
      in ''
        -- Create database ${dbName}
        CREATE DATABASE ${dbName};
        
        -- Create user ${dbConfig.user}
        CREATE USER ${dbConfig.user} WITH PASSWORD '$(cat ${passwordPath})';
        
        -- Grant privileges
        GRANT ALL PRIVILEGES ON DATABASE ${dbName} TO ${dbConfig.user};
        
        -- Connect to database and set permissions
        \c ${dbName}
        GRANT ALL ON SCHEMA public TO ${dbConfig.user};
        ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO ${dbConfig.user};
        ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO ${dbConfig.user};
        
      '') (builtins.attrNames databases)}
    '';
    
    authentication = ''
      # Database-specific authentication
      ${lib.concatMapStringsSep "\n" (dbName: let
        dbConfig = databases.${dbName};
      in ''
        host    ${dbName}        ${dbConfig.user}    127.0.0.1/32    md5
        host    ${dbName}        ${dbConfig.user}    ::1/128         md5
      '') (builtins.attrNames databases)}
      
      # Local admin access
      local   all             all                                 peer
      host    all             all             127.0.0.1/32        md5
      host    all             all             ::1/128             md5
    '';
    
    settings = {
      # Performance optimizations for multiple databases
      shared_buffers = "512MB";
      effective_cache_size = "2GB";
      maintenance_work_mem = "128MB";
      max_connections = 200;
      
      # Logging for better monitoring
      log_statement = "mod";
      log_min_duration_statement = 500;
      log_connections = true;
      log_disconnections = true;
      
      # Checkpoint settings
      checkpoint_completion_target = 0.9;
      checkpoint_timeout = "5min";
    };
  };

  # Create systemd tmpfiles for secret directories
  systemd.tmpfiles.rules = [
    "d /run/secrets 0755 root root -"
    "d /run/secrets/postgres 0755 postgres postgres -"
  ] ++ (lib.mapAttrsToList (dbName: _: 
    "d /run/secrets/postgres/${dbName} 0755 postgres postgres -"
  ) databases);
}
```

### PostgreSQL with Backup Integration

```nix
{ config, pkgs, ... }:

{
  services.onepassword-secrets = {
    enable = true;
    tokenFile = "/etc/opnix-token";
    
    secrets = {
      # PostgreSQL admin password
      postgresAdminPassword = {
        reference = "op://Homelab/PostgreSQL-Admin/password";
        owner = "postgres";
        group = "postgres";
        mode = "0600";
        services = ["postgresql"];
      };
      
      # Backup user password
      postgresBackupPassword = {
        reference = "op://Homelab/PostgreSQL-Backup/password";
        owner = "postgres";
        group = "postgres";
        mode = "0600";
        services = ["postgresql"];
      };
      
      # S3 credentials for backups
      backupS3Credentials = {
        reference = "op://Homelab/Backup-S3/credentials";
        path = "/etc/postgresql/s3-credentials";
        owner = "postgres";
        group = "postgres";
        mode = "0600";
        services = ["postgresql-backup"];
      };
    };
  };

  services.postgresql = {
    enable = true;
    package = pkgs.postgresql_15;
    
    initialScript = pkgs.writeText "postgres-backup-init.sql" ''
      -- Set admin password
      ALTER USER postgres PASSWORD '$(cat ${config.services.onepassword-secrets.secretPaths."postgres/admin-password"})';
      
      -- Create backup user
      CREATE USER backup_user WITH PASSWORD '$(cat ${config.services.onepassword-secrets.secretPaths."postgres/backup-password"})';
      
      -- Grant backup privileges
      GRANT SELECT ON ALL TABLES IN SCHEMA public TO backup_user;
      GRANT SELECT ON ALL SEQUENCES IN SCHEMA public TO backup_user;
      ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO backup_user;
    '';
    
    settings = {
      # Enable WAL archiving for continuous backup
      wal_level = "replica";
      archive_mode = true;
      archive_command = "/usr/local/bin/archive-wal.sh %f %p";
      max_wal_senders = 3;
      wal_keep_segments = 64;
    };
  };

  # Backup service
  systemd.services.postgresql-backup = {
    description = "PostgreSQL Database Backup";
    serviceConfig = {
      Type = "oneshot";
      User = "postgres";
      Group = "postgres";
      ExecStart = pkgs.writeScript "postgres-backup" ''
        #!/bin/bash
        set -euo pipefail
        
        # Load S3 credentials
        source ${config.services.onepassword-secrets.secretPaths."backup/s3-credentials"}
        
        # Set backup parameters
        BACKUP_DATE=$(date +%Y%m%d_%H%M%S)
        BACKUP_DIR="/var/backups/postgresql"
        BACKUP_FILE="$BACKUP_DIR/backup_$BACKUP_DATE.sql.gz"
        
        # Ensure backup directory exists
        mkdir -p "$BACKUP_DIR"
        
        # Create database backup
        echo "Creating PostgreSQL backup..."
        PGPASSWORD="$(cat ${config.services.onepassword-secrets.secretPaths."postgres/backup-password"})" \
        pg_dumpall -h localhost -U backup_user | gzip > "$BACKUP_FILE"
        
        # Upload to S3 (assuming aws-cli is configured with credentials)
        echo "Uploading backup to S3..."
        aws s3 cp "$BACKUP_FILE" "s3://my-backups/postgresql/" --storage-class GLACIER
        
        # Clean up local backups older than 7 days
        find "$BACKUP_DIR" -name "backup_*.sql.gz" -mtime +7 -delete
        
        echo "Backup completed successfully"
      '';
    };
    after = [ "postgresql.service" "opnix-secrets.service" ];
    wants = [ "postgresql.service" ];
  };

  # Schedule daily backups
  systemd.timers.postgresql-backup = {
    description = "PostgreSQL Backup Timer";
    wantedBy = [ "timers.target" ];
    timerConfig = {
      OnCalendar = "daily";
      Persistent = true;
      RandomizedDelaySec = "1h";
    };
  };

  # Create backup directory
  systemd.tmpfiles.rules = [
    "d /var/backups/postgresql 0755 postgres postgres -"
  ];
}
```

## Validation

### Check Database Setup

```bash
# Verify PostgreSQL is running
sudo systemctl status postgresql.service

# Check if secrets are deployed
sudo ls -la /run/secrets/postgres/
sudo ls -la /var/lib/postgresql/

# Test database connection with postgres user
sudo -u postgres psql -c "SELECT version();"

# List databases
sudo -u postgres psql -c "\l"

# List users
sudo -u postgres psql -c "\du"
```

### Test Database Authentication

```bash
# Test postgres user authentication
sudo -u postgres psql -c "SELECT current_user;"

# Test application user authentication
PGPASSWORD="$(sudo cat /run/secrets/postgres/app_user/password)" \
psql -h localhost -U myapp_user -d myapp_production -c "SELECT current_user;"

# Test SSL connection (if configured)
PGPASSWORD="password" psql "sslmode=require host=localhost user=myapp_user dbname=myapp_production"
```

### Verify Service Integration

```bash
# Check service dependencies
systemctl show postgresql.service -p After -p Wants

# Test secret updates
sudo systemctl restart opnix-secrets.service
sudo systemctl status postgresql.service

# Monitor logs during secret updates
sudo journalctl -u opnix-secrets.service -u postgresql.service -f
```

## Troubleshooting

### Database Won't Start

**Problem**: PostgreSQL fails to start after OpNix deployment.

**Solutions**:
1. Check PostgreSQL logs:
   ```bash
   sudo journalctl -u postgresql.service
   sudo tail -f /var/log/postgresql/postgresql-*.log
   ```

2. Verify secret files exist:
   ```bash
   sudo ls -la /run/secrets/postgres/
   ```

3. Test password file manually:
   ```bash
   sudo cat /run/secrets/postgres/superuser-password
   ```

### Authentication Failed

**Problem**: Cannot connect to database with OpNix-managed passwords.

**Solutions**:
1. Verify password was set correctly:
   ```bash
   sudo -u postgres psql -c "SELECT rolname FROM pg_roles WHERE rolname = 'postgres';"
   ```

2. Check authentication configuration:
   ```bash
   sudo cat /var/lib/postgresql/data/pg_hba.conf
   ```

3. Test connection manually:
   ```bash
   PGPASSWORD="$(sudo cat /run/secrets/postgres/superuser-password)" \
   psql -h localhost -U postgres -c "SELECT current_user;"
   ```

### SSL Certificate Issues

**Problem**: SSL connections fail with certificate errors.

**Solutions**:
1. Verify SSL files exist and have correct permissions:
   ```bash
   sudo ls -la /var/lib/postgresql/server.*
   sudo ls -la /var/lib/postgresql/ca.crt
   ```

2. Test certificate validity:
   ```bash
   sudo openssl x509 -in /var/lib/postgresql/server.crt -text -noout
   ```

3. Check PostgreSQL SSL configuration:
   ```bash
   sudo -u postgres psql -c "SHOW ssl;"
   sudo -u postgres psql -c "SHOW ssl_cert_file;"
   ```

### Backup Service Issues

**Problem**: Backup service fails to run.

**Solutions**:
1. Check backup service logs:
   ```bash
   sudo journalctl -u postgresql-backup.service
   ```

2. Test backup script manually:
   ```bash
   sudo -u postgres /path/to/backup/script
   ```

3. Verify backup user permissions:
   ```bash
   sudo -u postgres psql -c "\du backup_user"
   ```

## Security Considerations

1. **Password Security**: Use strong, unique passwords for all database users
2. **File Permissions**: Ensure secret files have restrictive permissions (0600)
3. **Network Security**: Configure pg_hba.conf to restrict network access
4. **SSL/TLS**: Enable SSL for all remote connections
5. **Backup Encryption**: Encrypt database backups before storing remotely
6. **Audit Logging**: Enable PostgreSQL audit logging for security monitoring
7. **Regular Updates**: Keep PostgreSQL and OpNix updated to latest versions

## Related Examples

- [Basic Home Manager Setup](./basic-home-manager.md) - User-level secret management
- [Caddy Web Server](./caddy-ssl.md) - SSL certificates for web services
- Check the [Examples Directory](./README.md) for more configuration examples