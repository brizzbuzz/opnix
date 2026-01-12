package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/brizzbuzz/opnix/internal/errors"
	"github.com/brizzbuzz/opnix/internal/onepass"
)

type command interface {
	Name() string
	Init([]string) error
	Run() error
}

func main() {
	cmds := []command{
		newSecretCommand(),
		newTokenCommand(),
		newEnvCommand(),
	}

	os.Exit(run(os.Args, cmds))
}

func printUsage(cmds []command) {
	fmt.Fprintf(os.Stderr, "Usage: opnix <command> [options]\n\n")
	fmt.Fprintf(os.Stderr, "Available commands:\n")
	fmt.Fprintf(os.Stderr, "  secret    Manage and retrieve secrets from 1Password\n")
	fmt.Fprintf(os.Stderr, "  token     Manage the 1Password service account token\n")
	fmt.Fprintf(os.Stderr, "  env       Resolve environment variables for development shells\n\n")
	fmt.Fprintf(os.Stderr, "Use 'opnix <command> -h' for command-specific help\n")
}

func run(args []string, cmds []command) int {
	if len(args) < 2 {
		printUsage(cmds)
		return 1
	}

	subcommand := args[1]

	for _, cmd := range cmds {
		if cmd.Name() == subcommand {
			if err := cmd.Init(args[2:]); err != nil {
				handleError(fmt.Errorf("failed to initialize %s: %w", cmd.Name(), err))
				return 1
			}
			if err := cmd.Run(); err != nil {
				handleError(err)
				return 1
			}
			return 0
		}
	}

	fmt.Fprintf(os.Stderr, "Unknown command: %s\n", subcommand)
	printUsage(cmds)
	return 1
}

// handleError provides user-friendly error output
func handleError(err error) {
	if err == nil {
		return
	}

	// Check if it's an OpnixError with structured information
	if opnixErr, ok := err.(*errors.OpnixError); ok {
		// Print structured error with full context
		fmt.Fprintf(os.Stderr, "%s\n", opnixErr.Error())
	} else {
		// Handle regular errors with some formatting
		errMsg := err.Error()

		// Add some context for common error patterns
		if strings.Contains(errMsg, "no such file or directory") {
			fmt.Fprintf(os.Stderr, "ERROR: File not found\n")
			fmt.Fprintf(os.Stderr, "  %s\n", errMsg)
			fmt.Fprintf(os.Stderr, "\n  Suggestions:\n")
			fmt.Fprintf(os.Stderr, "  1. Check the file path is correct\n")
			fmt.Fprintf(os.Stderr, "  2. Verify the file exists: ls -la <path>\n")
		} else if strings.Contains(errMsg, "permission denied") {
			fmt.Fprintf(os.Stderr, "ERROR: Permission denied\n")
			fmt.Fprintf(os.Stderr, "  %s\n", errMsg)
			fmt.Fprintf(os.Stderr, "\n  Suggestions:\n")
			fmt.Fprintf(os.Stderr, "  1. Check file/directory permissions\n")
			fmt.Fprintf(os.Stderr, "  2. Run with appropriate privileges if needed\n")
		} else {
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", errMsg)
		}
	}
}

const defaultEnvConfigPath = "opnix-env.json"

type envCommand struct {
	fs *flag.FlagSet

	configPath string
	tokenFile  string
	format     string

	loadConfig func(string) (*envConfig, error)
	newClient  func(string) (secretResolver, error)
}

type secretResolver interface {
	ResolveSecret(string) (string, error)
}

type envConfig struct {
	Vars   []envVariable `json:"vars"`
	Format string        `json:"format,omitempty"`
}

type envVariable struct {
	Name               string `json:"name"`
	Reference          string `json:"reference,omitempty"`
	Value              string `json:"value,omitempty"`
	Optional           bool   `json:"optional,omitempty"`
	PreserveWhitespace bool   `json:"preserveWhitespace,omitempty"`
	Description        string `json:"description,omitempty"`
}

func (v envVariable) shouldTrim() bool {
	return !v.PreserveWhitespace
}

type envProcessor struct {
	resolver secretResolver
}

type envSkippedVariable struct {
	Name string
	Err  error
}

type envResult struct {
	Values  map[string]string
	Skipped []envSkippedVariable
}

func newEnvCommand() *envCommand {
	cmd := &envCommand{
		fs: flag.NewFlagSet("env", flag.ExitOnError),
	}

	cmd.fs.StringVar(&cmd.configPath, "config", defaultEnvConfigPath, "Path to environment configuration file")
	cmd.fs.StringVar(&cmd.tokenFile, "token-file", defaultTokenPath, "Path to file containing 1Password service account token")
	cmd.fs.StringVar(&cmd.format, "format", "", "Output format: shell (default), dotenv, json")

	cmd.fs.Usage = func() {
		fmt.Fprintf(cmd.fs.Output(), "Usage: opnix env [options]\n\n")
		fmt.Fprintf(cmd.fs.Output(), "Resolve environment variables from 1Password references\n\n")
		fmt.Fprintf(cmd.fs.Output(), "Options:\n")
		cmd.fs.PrintDefaults()
	}

	cmd.loadConfig = loadEnvConfig
	cmd.newClient = func(path string) (secretResolver, error) {
		return onepass.NewClient(path)
	}

	return cmd
}

func (e *envCommand) Name() string { return e.fs.Name() }

func (e *envCommand) Init(args []string) error {
	return e.fs.Parse(args)
}

func (e *envCommand) Run() error {
	if err := e.validateConfigFile(); err != nil {
		return err
	}

	cfg, err := e.loadConfig(e.configPath)
	if err != nil {
		return err
	}

	format := e.format
	if format == "" {
		if cfg.Format != "" {
			format = cfg.Format
		} else {
			format = "shell"
		}
	}

	format = strings.ToLower(format)
	if !isSupportedFormat(format) {
		return errors.ConfigValidationError(
			"env.format",
			format,
			"Unsupported format specified",
			[]string{
				"Use one of: shell, dotenv, json",
				"Example: opnix env -format shell",
			},
		)
	}

	resolver, err := e.buildResolver(cfg)
	if err != nil {
		return err
	}

	result, err := newEnvProcessor(resolver).Process(cfg)
	if err != nil {
		return err
	}

	for _, skipped := range result.Skipped {
		fmt.Fprintf(os.Stderr, "WARNING: Skipped optional env var %s: %v\n", skipped.Name, skipped.Err)
	}

	output, err := renderOutput(result.Values, format)
	if err != nil {
		return err
	}

	fmt.Print(output)
	return nil
}

func (e *envCommand) validateConfigFile() error {
	if _, err := os.Stat(e.configPath); err != nil {
		if os.IsNotExist(err) {
			return errors.FileOperationError(
				"Checking environment configuration file",
				e.configPath,
				"Environment configuration file does not exist",
				err,
			)
		}

		return errors.FileOperationError(
			"Checking environment configuration file",
			e.configPath,
			"Failed to access environment configuration file",
			err,
		)
	}
	return nil
}

func (e *envCommand) buildResolver(cfg *envConfig) (secretResolver, error) {
	for _, variable := range cfg.Vars {
		if variable.Reference != "" {
			return e.newClient(e.tokenFile)
		}
	}
	return staticResolver{}, nil
}

func newEnvProcessor(resolver secretResolver) *envProcessor {
	return &envProcessor{resolver: resolver}
}

func (p *envProcessor) Process(cfg *envConfig) (*envResult, error) {
	if cfg == nil {
		return nil, errors.ConfigError(
			"Processing environment configuration",
			"Environment configuration cannot be nil",
			nil,
		)
	}

	result := &envResult{
		Values:  make(map[string]string),
		Skipped: []envSkippedVariable{},
	}

	for i, variable := range cfg.Vars {
		value, err := p.resolveVariable(variable, i)
		if err != nil {
			if variable.Optional {
				result.Skipped = append(result.Skipped, envSkippedVariable{
					Name: variable.Name,
					Err:  err,
				})
				continue
			}
			return nil, err
		}
		result.Values[variable.Name] = value
	}

	return result, nil
}

func (p *envProcessor) resolveVariable(variable envVariable, index int) (string, error) {
	if variable.Reference != "" {
		value, err := p.resolver.ResolveSecret(variable.Reference)
		if err != nil {
			return "", errors.WrapWithSuggestions(
				err,
				fmt.Sprintf("Resolving secret for env var %s", variable.Name),
				"environment variable resolution",
				[]string{
					fmt.Sprintf("Check that the 1Password reference '%s' exists", variable.Reference),
					"Ensure the service account has access to the vault and item",
				},
			)
		}

		if variable.shouldTrim() {
			return strings.TrimSpace(value), nil
		}
		return value, nil
	}

	if variable.Value != "" {
		if variable.shouldTrim() {
			return strings.TrimSpace(variable.Value), nil
		}
		return variable.Value, nil
	}

	return "", errors.ConfigError(
		fmt.Sprintf("Processing env variable at index %d", index),
		"Variable must define either 'reference' or 'value'",
		nil,
	)
}

var envNamePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

func loadEnvConfig(path string) (*envConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.FileOperationError(
			"Loading environment configuration",
			path,
			"Failed to read environment config file",
			err,
		)
	}

	var cfg envConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, errors.ConfigError(
			"Parsing environment configuration",
			"Invalid JSON format in environment config file",
			err,
		)
	}

	if err := validateEnvConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func validateEnvConfig(cfg *envConfig) error {
	if len(cfg.Vars) == 0 {
		return errors.ConfigValidationError(
			"env.vars",
			"<empty>",
			"Environment configuration must define at least one variable",
			[]string{
				"Add variables under the 'vars' array",
				"Example: {\"vars\": [{\"name\": \"API_TOKEN\", \"reference\": \"op://Vault/Item/token\"}]}",
			},
		)
	}

	for i, variable := range cfg.Vars {
		fieldPrefix := fmt.Sprintf("env.vars[%d]", i)

		if variable.Name == "" {
			return errors.ConfigValidationError(
				fieldPrefix+".name",
				"<empty>",
				"Environment variable name cannot be empty",
				[]string{
					"Provide a unique uppercase variable name",
					"Example: API_TOKEN",
				},
			)
		}

		if !envNamePattern.MatchString(variable.Name) {
			return errors.ConfigValidationError(
				fieldPrefix+".name",
				variable.Name,
				"Environment variable names must use uppercase letters, numbers, and underscores",
				[]string{
					"Start with an uppercase letter",
					"Use uppercase letters, digits, and underscores only",
					"Example: DATABASE_PASSWORD",
				},
			)
		}

		hasReference := variable.Reference != ""
		hasValue := variable.Value != ""

		if hasReference && hasValue {
			return errors.ConfigValidationError(
				fieldPrefix,
				variable.Name,
				"Specify either 'reference' or 'value', not both",
				[]string{
					"Remove the 'value' field to use a 1Password reference",
					"Or remove the 'reference' field to use a static value",
				},
			)
		}

		if !hasReference && !hasValue {
			return errors.ConfigValidationError(
				fieldPrefix,
				variable.Name,
				"Environment variable must define a 1Password reference or static value",
				[]string{
					"Add a 'reference': \"op://Vault/Item/field\"",
					"Or add a 'value' for static configuration",
				},
			)
		}
	}

	return nil
}

func isSupportedFormat(format string) bool {
	switch format {
	case "shell", "dotenv", "json":
		return true
	default:
		return false
	}
}

type staticResolver struct{}

func (staticResolver) ResolveSecret(string) (string, error) {
	return "", fmt.Errorf("no 1Password client configured")
}

func renderOutput(values map[string]string, format string) (string, error) {
	switch format {
	case "shell":
		return renderShell(values), nil
	case "dotenv":
		return renderDotenv(values), nil
	case "json":
		data, err := json.MarshalIndent(values, "", "  ")
		if err != nil {
			return "", errors.ConfigError(
				"Rendering environment variables",
				"Failed to marshal JSON output",
				err,
			)
		}
		return string(data) + "\n", nil
	default:
		return "", fmt.Errorf("unknown format: %s", format)
	}
}

func renderShell(values map[string]string) string {
	var keys []string
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, key := range keys {
		fmt.Fprintf(&b, "export %s=%s\n", key, shellQuote(values[key]))
	}
	return b.String()
}

func renderDotenv(values map[string]string) string {
	var keys []string
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, key := range keys {
		fmt.Fprintf(&b, "%s=%s\n", key, dotenvValue(values[key]))
	}

	return b.String()
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func dotenvValue(value string) string {
	if value == "" {
		return ""
	}

	if strings.ContainsAny(value, " #\"'\n\r\t") {
		escaped := strings.ReplaceAll(value, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		escaped = strings.ReplaceAll(escaped, "\n", `\n`)
		escaped = strings.ReplaceAll(escaped, "\r", `\r`)
		escaped = strings.ReplaceAll(escaped, "\t", `\t`)
		return `"` + escaped + `"`
	}

	return value
}
