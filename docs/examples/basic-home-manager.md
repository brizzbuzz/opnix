# Basic Home Manager Setup with OpNix

This example demonstrates how to set up OpNix with Home Manager for managing user-specific secrets like SSH keys, API tokens, and configuration files.

## Prerequisites

- Home Manager configured and working
- 1Password service account with access to personal secrets
- Personal secrets stored in 1Password (SSH keys, API tokens, etc.)
- OpNix token accessible to your user account

## 1Password Setup

Create items in your 1Password vault for personal secrets:

### SSH Key Item
```
Vault: Personal
Item: SSH-Key-Main
Fields:
  - private-key: [SSH private key content]
  - public-key: [SSH public key content]
  - passphrase: [key passphrase if any]
  - notes: Main SSH key for development
```

### API Tokens Item
```
Vault: Personal
Item: Development-API-Keys
Fields:
  - github-token: [GitHub personal access token]
  - openai-api-key: [OpenAI API key]
  - docker-hub-token: [Docker Hub access token]
```

### Configuration Files Item
```
Vault: Personal
Item: Development-Config
Fields:
  - gpg-key: [GPG private key]
  - npmrc: [NPM configuration with tokens]
  - gitconfig-signing-key: [Git signing key ID]
```

## Configuration

### Basic Home Manager Setup

```nix
{ config, pkgs, ... }:

{
  # Enable OpNix for Home Manager
  programs.onepassword-secrets = {
    enable = true;
    tokenFile = "/etc/opnix-token";  # System token (requires group membership)
    
    secrets = {
      # SSH private key
      sshPrivateKey = {
        reference = "op://Personal/SSH-Key-Main/private-key";
        path = ".ssh/id_rsa";
        mode = "0600";
      };
      
      # SSH public key
      sshPublicKey = {
        reference = "op://Personal/SSH-Key-Main/public-key";
        path = ".ssh/id_rsa.pub";
        mode = "0644";
      };
      
      # GitHub API token
      githubToken = {
        reference = "op://Personal/Development-API-Keys/github-token";
        path = ".config/gh/token";
        mode = "0600";
      };
      
      # GPG private key
      gpgPrivateKey = {
        reference = "op://Personal/Development-Config/gpg-key";
        path = ".gnupg/private-key.asc";
        mode = "0600";
      };
      
      # NPM configuration
      npmConfig = {
        reference = "op://Personal/Development-Config/npmrc";
        path = ".npmrc";
        mode = "0600";
      };
    };
  };

  # Configure SSH with the managed key
  programs.ssh = {
    enable = true;
    
    # SSH will automatically use ~/.ssh/id_rsa
    extraConfig = ''
      # Use OpNix-managed SSH key
      IdentityFile ~/.ssh/id_rsa
      
      # GitHub configuration
      Host github.com
        HostName github.com
        User git
        IdentityFile ~/.ssh/id_rsa
        IdentitiesOnly yes
    '';
    
    # SSH agent configuration
    startAgent = true;
  };

  # Configure Git with signing key
  programs.git = {
    enable = true;
    userName = "Your Name";
    userEmail = "your.email@example.com";
    
    # Git signing configuration will reference the GPG key
    extraConfig = {
      commit = {
        gpgsign = true;
      };
      user = {
        signingkey = "your-gpg-key-id";  # Replace with actual key ID
      };
    };
  };

  # Configure GitHub CLI
  programs.gh = {
    enable = true;
    # GitHub CLI will use the token from ~/.config/gh/token
  };

  # Ensure necessary directories exist
  home.file = {
    ".ssh/.keep".text = "";
    ".config/gh/.keep".text = "";
    ".gnupg/.keep".text = "";
  };
}
```

### User-Specific Token Setup

If you prefer to use a user-specific token instead of the system token:

```nix
{ config, pkgs, ... }:

{
  programs.onepassword-secrets = {
    enable = true;
    # Use user-specific token
    tokenFile = "${config.home.homeDirectory}/.config/opnix/token";
    
    secrets = {
      sshPrivateKey = {
        reference = "op://Personal/SSH-Key-Main/private-key";
        path = ".ssh/id_rsa";
        mode = "0600";
      };
      
      # Add more secrets as needed
    };
  };

  # Create token directory
  home.file.".config/opnix/.keep".text = "";
}
```

### Advanced Multi-Service Configuration

```nix
{ config, pkgs, ... }:

{
  programs.onepassword-secrets = {
    enable = true;
    tokenFile = "/etc/opnix-token";
    
    secrets = {
      # Development SSH keys
      sshPrivateKey = {
        reference = "op://Personal/SSH-Key-Main/private-key";
        path = ".ssh/id_rsa";
        mode = "0600";
      };
      
      sshEd25519Key = {
        reference = "op://Personal/SSH-Key-Ed25519/private-key";
        path = ".ssh/id_ed25519";
        mode = "0600";
      };
      
      # API tokens
      githubToken = {
        reference = "op://Personal/Development-API-Keys/github-token";
        path = ".config/tokens/github";
        mode = "0600";
      };
      
      openaiApiKey = {
        reference = "op://Personal/Development-API-Keys/openai-api-key";
        path = ".config/tokens/openai";
        mode = "0600";
      };
      
      dockerHubToken = {
        reference = "op://Personal/Development-API-Keys/docker-hub-token";
        path = ".config/tokens/docker-hub";
        mode = "0600";
      };
      
      # Configuration files
      npmConfig = {
        reference = "op://Personal/Development-Config/npmrc";
        path = ".npmrc";
        mode = "0600";
      };
      
      pypiConfig = {
        reference = "op://Personal/Development-Config/pypirc";
        path = ".pypirc";
        mode = "0600";
      };
      
      # Database connection strings for development
      databaseUrl = {
        reference = "op://Personal/Development-DB/connection-string";
        path = ".config/dev/database-url";
        mode = "0600";
      };
      
      # Cloud provider credentials
      awsCredentials = {
        reference = "op://Personal/AWS-Development/credentials";
        path = ".aws/credentials";
        mode = "0600";
      };
    };
  };

  # Configure multiple SSH keys
  programs.ssh = {
    enable = true;
    extraConfig = ''
      # Default key
      IdentityFile ~/.ssh/id_rsa
      
      # Ed25519 key for specific hosts
      Host secure-server
        HostName secure-server.example.com
        User admin
        IdentityFile ~/.ssh/id_ed25519
        IdentitiesOnly yes
      
      # GitHub
      Host github.com
        HostName github.com
        User git
        IdentityFile ~/.ssh/id_rsa
        IdentitiesOnly yes
      
      # GitLab
      Host gitlab.com
        HostName gitlab.com
        User git
        IdentityFile ~/.ssh/id_ed25519
        IdentitiesOnly yes
    '';
  };

  # Configure Git with multiple identities
  programs.git = {
    enable = true;
    userName = "Your Name";
    userEmail = "your.email@example.com";
    
    # Include additional Git configuration
    includes = [
      {
        condition = "gitdir:~/work/";
        contents = {
          user = {
            email = "work.email@company.com";
            signingkey = "work-gpg-key-id";
          };
        };
      }
    ];
    
    extraConfig = {
      commit.gpgsign = true;
      pull.rebase = true;
      init.defaultBranch = "main";
    };
  };

  # Development environment variables
  home.sessionVariables = {
    # Reference OpNix-managed secrets in environment
    GITHUB_TOKEN = "$(cat ${config.home.homeDirectory}/.config/tokens/github)";
    OPENAI_API_KEY = "$(cat ${config.home.homeDirectory}/.config/tokens/openai)";
    DATABASE_URL = "$(cat ${config.home.homeDirectory}/.config/dev/database-url)";
  };

  # Development packages that might use the secrets
  home.packages = with pkgs; [
    gh              # GitHub CLI (uses GitHub token)
    docker          # Docker (uses Docker Hub token)
    awscli2         # AWS CLI (uses AWS credentials)
    nodejs          # Node.js (uses .npmrc)
    python3         # Python (uses .pypirc)
  ];

  # Create necessary directories
  home.file = {
    ".ssh/.keep".text = "";
    ".config/tokens/.keep".text = "";
    ".config/dev/.keep".text = "";
    ".aws/.keep".text = "";
  };
}
```

### Development Workflow Integration

```nix
{ config, pkgs, ... }:

{
  programs.onepassword-secrets = {
    enable = true;
    tokenFile = "/etc/opnix-token";
    
    secrets = {
      # SSH key for development
      sshDevKey = {
        reference = "op://Personal/SSH-Dev-Key/private-key";
        path = ".ssh/id_rsa";
        mode = "0600";
      };
      
      # Development environment file
      devEnvFile = {
        reference = "op://Personal/Development-Environment/env-file";
        path = ".config/dev/.env";
        mode = "0600";
      };
      
      # API keys for development tools
      devApiKeys = {
        reference = "op://Personal/Development-API-Keys/all-keys";
        path = ".config/dev/api-keys";
        mode = "0600";
      };
    };
  };

  # Development shell configuration
  programs.zsh = {
    enable = true;
    
    # Load development environment
    initExtra = ''
      # Load development environment variables
      if [ -f ~/.config/dev/.env ]; then
        export $(grep -v '^#' ~/.config/dev/.env | xargs)
      fi
      
      # Load API keys
      if [ -f ~/.config/dev/api-keys ]; then
        source ~/.config/dev/api-keys
      fi
      
      # Helper function to reload secrets
      reload-secrets() {
        echo "Reloading Home Manager secrets..."
        home-manager switch
        echo "Secrets reloaded!"
      }
    '';
    
    # Aliases for development
    shellAliases = {
      # Git aliases using SSH key
      gp = "git push";
      gpl = "git pull";
      gs = "git status";
      
      # Docker aliases using Docker Hub token
      dlogin = "cat ~/.config/tokens/docker-hub | docker login --username your-username --password-stdin";
    };
  };

  # Configure direnv for project-specific environments
  programs.direnv = {
    enable = true;
    nix-direnv.enable = true;
  };

  # Development tools that integrate with secrets
  programs.vscode = {
    enable = true;
    
    # VS Code will automatically pick up Git configuration
    # and SSH keys from Home Manager
  };
}
```

## Setup Instructions

### 1. Set Up Group Membership (for System Token)

```bash
# Add your user to the onepassword-secrets group
sudo usermod -a -G onepassword-secrets $USER

# Log out and log back in, or use newgrp
newgrp onepassword-secrets

# Verify group membership
groups $USER
```

### 2. Set Up User-Specific Token (Alternative)

```bash
# Create token directory
mkdir -p ~/.config/opnix

# Set the token (you'll be prompted for it)
opnix token set -path ~/.config/opnix/token

# Verify token permissions
ls -la ~/.config/opnix/token
# Should show: -rw------- 1 yourusername yourusername
```

### 3. Apply Home Manager Configuration

```bash
# Apply the configuration
home-manager switch

# Verify secrets are deployed
ls -la ~/.ssh/
ls -la ~/.config/tokens/
ls -la ~/.npmrc
```

## Validation

### Check Secret Deployment

```bash
# Verify SSH key is deployed
ls -la ~/.ssh/id_rsa
file ~/.ssh/id_rsa  # Should show "PEM RSA private key"

# Check file permissions
stat -c "%a %n" ~/.ssh/id_rsa  # Should show "600"
stat -c "%a %n" ~/.ssh/id_rsa.pub  # Should show "644"

# Verify API tokens
ls -la ~/.config/tokens/
cat ~/.config/tokens/github | head -c 20  # Should show token prefix
```

### Test SSH Configuration

```bash
# Test SSH key loading
ssh-add -l

# Test SSH connection to GitHub
ssh -T git@github.com

# Test Git operations
cd /tmp
git clone git@github.com:yourusername/test-repo.git
```

### Test API Token Usage

```bash
# Test GitHub CLI with token
gh auth status

# Test NPM with .npmrc
npm whoami

# Test environment variables
echo $GITHUB_TOKEN | head -c 20
```

## Troubleshooting

### Secrets Not Deployed

**Problem**: Home Manager completes but secrets aren't in expected locations.

**Solutions**:
1. Check Home Manager logs:
   ```bash
   journalctl --user -u home-manager-$USER.service
   ```

2. Check OpNix token access:
   ```bash
   # For system token
   cat /etc/opnix-token
   groups $USER | grep onepassword-secrets
   
   # For user token
   cat ~/.config/opnix/token
   ```

3. Run Home Manager with verbose output:
   ```bash
   home-manager switch --verbose
   ```

### Permission Denied Accessing Token

**Problem**: Cannot read system token file.

**Solutions**:
1. Verify group membership:
   ```bash
   groups $USER
   id $USER
   ```

2. Check token file permissions:
   ```bash
   ls -la /etc/opnix-token
   # Should show: -rw-r----- 1 root onepassword-secrets
   ```

3. Add user to group and relogin:
   ```bash
   sudo usermod -a -G onepassword-secrets $USER
   # Then logout and login again
   ```

### SSH Key Not Working

**Problem**: SSH authentication fails with OpNix-managed key.

**Solutions**:
1. Check key file format:
   ```bash
   file ~/.ssh/id_rsa
   head -1 ~/.ssh/id_rsa  # Should start with "-----BEGIN"
   ```

2. Test key loading:
   ```bash
   ssh-keygen -y -f ~/.ssh/id_rsa  # Should output public key
   ```

3. Add key to SSH agent:
   ```bash
   ssh-add ~/.ssh/id_rsa
   ssh-add -l
   ```

### API Tokens Not Working

**Problem**: Applications can't use API tokens.

**Solutions**:
1. Verify token content:
   ```bash
   cat ~/.config/tokens/github | wc -c  # Should have reasonable length
   cat ~/.config/tokens/github | head -c 10  # Check prefix
   ```

2. Test token manually:
   ```bash
   # Test GitHub token
   curl -H "Authorization: token $(cat ~/.config/tokens/github)" \
        https://api.github.com/user
   ```

3. Check application configuration:
   ```bash
   # Verify GitHub CLI configuration
   gh auth status
   cat ~/.config/gh/hosts.yml
   ```

## Security Considerations

1. **File Permissions**: Ensure private keys and tokens have 0600 permissions
2. **Token Storage**: Choose between system token (requires group) or user token
3. **SSH Agent**: Use SSH agent to avoid entering passphrases repeatedly
4. **Environment Variables**: Be careful when using secrets in environment variables
5. **Git Commits**: Never commit actual secrets to version control
6. **Backup Strategy**: Ensure 1Password is properly backed up
7. **Token Rotation**: Regularly rotate API tokens and SSH keys

## Related Examples

- [Basic NixOS Setup](./basic-nixos.md) - System-level secret management
- [Basic nix-darwin Setup](./basic-darwin.md) - macOS system secrets
- [Multi-Environment Setup](./multi-environment.md) - Managing secrets across environments
- [Development Environment](./macos-development.md) - Advanced development setup