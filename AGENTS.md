# Repository Guidelines

## Project Structure & Module Organization
OpNix couples a Go CLI with Nix packaging. The CLI entry point lives in `cmd/opnix/` (split into `main.go`, `secret.go`, and `token.go` with unit tests). Reusable logic sits under `internal/`—`config` for module parsing, `onepass` for 1Password API access, `secrets` and `systemd` for OS integration, and `validation` for input checks. Nix modules and dev tooling reside in `nix/`, while user-facing guides live in `docs/`. Keep the generated `opnix` binary out of commits.

## Build, Test, and Development Commands
Enter the development environment with `nix develop`. Build the CLI using `go build ./cmd/opnix`, or produce the packaged binary with `nix build .#opnix`. Run the test suite via `go test ./...`, lint with `golangci-lint run ./...`, and finish with `nix flake check` to verify formatting and Nix evaluations.

## Coding Style & Naming Conventions
All Go files must be `gofmt`-clean (tabs for indentation, imports ordered by `goimports`). Exported types and functions need sentence-case doc comments, and test helpers stay unexported unless reused across packages. Follow snake-case for CLI flags and hyphenated command names, mirroring the existing subcommands. Nix expressions should be formatted with `alejandra`, and new modules should mirror the `nix/module.nix` pattern for options and documentation.

## Testing Guidelines
Place unit tests alongside implementation files using the `_test.go` suffix and table-driven patterns for input variance. When adding behavior that touches external services, prefer fakes in `internal/onepass` over real network calls. Run `go test ./... -count=1` before submission. For module or packaging changes, include `nix flake check` output in your PR description and add example snippets under `docs/examples/` when behavior changes.

## Commit & Pull Request Guidelines
The project follows Conventional Commits (`feat:`, `fix:`, `refactor:`, etc.) with issue cross-links when relevant (e.g., `fix: adjust permissions (#30)`). Commits should be scoped narrowly and leave the tree buildable. Pull requests must include a concise summary, testing notes (at minimum `go test ./...` and `nix flake check`), and screenshots or CLI excerpts when user-visible output changes. Request review from maintainers familiar with the touched package or module and wait for green CI before merging.

## Security & Configuration Tips
Never commit 1Password tokens or sample vault data; rely on `sudo opnix token set` during manual testing. Treat `/etc/opnix-token` and generated secret files as sensitive, and scrub them from reproductions. When documenting configuration, use sanitized vault paths (e.g., `op://Example/Service/password`) and remind users to rotate tokens after testing.
