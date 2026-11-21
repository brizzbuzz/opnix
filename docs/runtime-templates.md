# Runtime templates

Runtime templates combine named 1Password field secrets with literal text in an output file. OpNix resolves secrets when it runs, not during Nix evaluation. Template content and marker text can be stored in the world-readable Nix store, so never put credentials directly in `content`.

## Plain text with several secrets

Each named secret has its own placeholder. This Home Manager example writes both values into `~/.config/myapp/credentials.txt`, alongside ordinary text:

```nix
{ config, ... }: {
  programs.onepassword-secrets = {
    enable = true;
    secrets = {
      databasePassword.reference = "op://Example/Database/password";
      apiToken.reference = "op://Example/Service/token";
    };
    templates.credentials = {
      path = ".config/myapp/credentials.txt";
      content = ''
        Application credentials
        Database password: ${config.programs.onepassword-secrets.placeholder.databasePassword}
        API token: ${config.programs.onepassword-secrets.placeholder.apiToken}
      '';
    };
  };
}
```

Store the actual password and token in those 1Password fields. No base64 encoding is needed for this plain-text example. Each placeholder is replaced with its field's exact value at runtime. A placeholder can appear more than once, and newlines in a value are preserved.

## Environment-style file

Using the same secret definitions, add this template inside `programs.onepassword-secrets` for an application that reads `KEY=value` files:

```nix
templates.appEnv = {
  path = ".config/myapp/app.env";
  content = ''
    DATABASE_PASSWORD=${config.programs.onepassword-secrets.placeholder.databasePassword}
    API_TOKEN=${config.programs.onepassword-secrets.placeholder.apiToken}
  '';
};
```

This assumes single-line values accepted literally by the application's parser. OpNix does not quote or escape them. Do not source the file as shell code unless the values are safe for shell syntax.

## Docker JSON

```nix
{ config, ... }: {
  programs.onepassword-secrets = {
    enable = true;
    secrets.dockerhubAuth.reference = "op://Example/Docker Hub/auth";
    templates.dockerConfig = {
      path = ".docker/config.json";
      content = builtins.toJSON {
        auths."https://index.docker.io/v1/".auth = config.programs.onepassword-secrets.placeholder.dockerhubAuth;
      };
    };
  };
}
```

The 1Password field must already contain the registry's base64-encoded `username:password` auth value. OpNix does not encode it. `builtins.toJSON` escapes the marker, not the runtime secret. This example is suitable for that base64 value, not arbitrary JSON strings.

Each template requires `path` and `content`. Relative Home Manager paths are relative to the home directory; on NixOS and nix-darwin they are relative to `outputDir`. Absolute paths are used as given. The read-only `templatePaths.dockerConfig` exposes the resolved path. `placeholder.dockerhubAuth` is a read-only marker derived from the `secrets.dockerhubAuth` attribute name. Only named field secrets have markers; file secrets cannot be used in templates. Source secrets are still written as standalone files.

The default template mode is `0600`. NixOS and nix-darwin default to owner and group `root`; Home Manager defaults to the home user and group `users` on Linux or `staff` on macOS. Set `owner`, `group`, or `mode` on a template to override them. Modes permit ordinary octal permission bits `000` through `0777`, without special bits.

## Plain text and NixOS services

NixOS and nix-darwin use `services.onepassword-secrets` instead of the Home Manager namespace. For example, this NixOS module renders a multiline text file:

```nix
{ config, ... }: {
  services.onepassword-secrets = {
    enable = true;
    secrets.apiToken.reference = "op://Example/Service/token";
    templates.appConfig = {
      path = "app/settings.txt";
      content = ''
        Application settings
        token: ${config.services.onepassword-secrets.placeholder.apiToken}
        End of settings
      '';
      services.myapp = { restart = false; signal = "SIGHUP"; };
    };
  };
}
```

Replacement is literal, one-pass text substitution. OpNix does not parse the output or recursively expand markers contained in a secret value. It applies no format-aware escaping: quotes, backslashes, or newlines in a value can invalidate JSON, YAML, TOML, or shell syntax. Choose a format and secret values appropriate for the consuming application.

On NixOS, template `services` accepts the existing list of service names or an attribute set with actions such as `restart` and `signal`. With systemd integration and change detection configured, changes to rendered template output participate in hashing and service handling. nix-darwin accepts a list of service names; Home Manager has no per-template `services` option.

## JSON configuration

The CLI JSON schema has a `secrets` array and a separate `templates` array. A named field secret and its marker must be available in the same invocation. For example, the marker below is SHA-256 of the UTF-8 name `defaultSecret`:

```json
{
  "secrets": [
    { "name": "defaultSecret", "path": "source", "reference": "op://Example/Service/password", "kind": "field" }
  ],
  "templates": [
    { "name": "appConfig", "path": "app.conf", "content": "password=<OPNIX:7ab73f276766e0ae10ed5dbd2bad50a3db7dda14a35a05a91df135dd3769adfa>" }
  ]
}
```

Markers have the deterministic form `<OPNIX:hex>`, where `hex` is the lowercase SHA-256 digest of the secret name. Names identify secrets, not paths or references. Nix modules invoke `configFiles` separately, so declarative templates require declarative source secrets; they cannot refer to sources in a separate `configFiles` invocation. There is no template-only source mode.

## Write behavior

Resolution and rendering are preflighted before output files are changed. If either fails, output files remain unchanged. A later filesystem write failure is not a global transaction: earlier outputs may already have changed. Each template is written through a temporary file and renamed, preserving its old destination if that template's write fails. OpNix rejects a final destination symlink and reconciles mode and ownership even when content is unchanged. It does not guarantee protection against hostile races in parent directories; use trusted output directories.
