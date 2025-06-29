package systemd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/brizzbuzz/opnix/internal/config"
	"github.com/brizzbuzz/opnix/internal/errors"
)

// ServiceAction defines how to handle a service when secrets change
type ServiceAction struct {
	Name    string
	Restart bool
	Signal  string
	After   []string
}

// SecretHash represents a stored hash of a secret's content
type SecretHash struct {
	Path         string    `json:"path"`
	Hash         string    `json:"hash"`
	LastModified time.Time `json:"lastModified"`
}

// HashStore manages secret content hashes for change detection
type HashStore struct {
	Hashes   map[string]SecretHash `json:"hashes"`
	filePath string
}

// Manager handles systemd service integration and change detection
type Manager struct {
	config    config.SystemdIntegration
	hashStore *HashStore
	dryRun    bool
	systemctl string
}

// NewManager creates a new systemd integration manager
func NewManager(cfg config.SystemdIntegration) (*Manager, error) {
	// Find systemctl binary
	systemctl, err := exec.LookPath("systemctl")
	if err != nil {
		return nil, errors.FileOperationError(
			"Finding systemctl binary",
			"systemctl",
			"systemctl not found in PATH - systemd integration requires systemd",
			err,
		)
	}

	// Initialize hash store if change detection is enabled
	var hashStore *HashStore
	if cfg.ChangeDetection.Enable {
		hashStore, err = NewHashStore(cfg.ChangeDetection.HashFile)
		if err != nil {
			return nil, err
		}
	}

	return &Manager{
		config:    cfg,
		hashStore: hashStore,
		systemctl: systemctl,
	}, nil
}

// NewHashStore creates or loads a hash store from disk
func NewHashStore(filePath string) (*HashStore, error) {
	store := &HashStore{
		Hashes:   make(map[string]SecretHash),
		filePath: filePath,
	}

	// Create parent directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, errors.FileOperationError(
			"Creating hash store directory",
			filepath.Dir(filePath),
			"Failed to create directory for hash store",
			err,
		)
	}

	// Load existing hashes if file exists
	if _, err := os.Stat(filePath); err == nil {
		if err := store.load(); err != nil {
			return nil, err
		}
	}

	return store, nil
}

// load reads the hash store from disk
func (hs *HashStore) load() error {
	data, err := os.ReadFile(hs.filePath)
	if err != nil {
		return errors.FileOperationError(
			"Loading hash store",
			hs.filePath,
			"Failed to read hash store file",
			err,
		)
	}

	if err := json.Unmarshal(data, hs); err != nil {
		return errors.ConfigError(
			"Parsing hash store",
			"Invalid JSON format in hash store file",
			err,
		)
	}

	return nil
}

// save writes the hash store to disk
func (hs *HashStore) save() error {
	data, err := json.MarshalIndent(hs, "", "  ")
	if err != nil {
		return errors.ConfigError(
			"Serializing hash store",
			"Failed to marshal hash store data",
			err,
		)
	}

	if err := os.WriteFile(hs.filePath, data, 0644); err != nil {
		return errors.FileOperationError(
			"Saving hash store",
			hs.filePath,
			"Failed to write hash store file",
			err,
		)
	}

	return nil
}

// calculateHash calculates SHA-256 hash of a file's content
func (hs *HashStore) calculateHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", errors.FileOperationError(
			"Opening file for hashing",
			filePath,
			"Failed to open file for hash calculation",
			err,
		)
	}
	defer func() { _ = file.Close() }() // Ignore error - defer cleanup is best effort

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", errors.FileOperationError(
			"Reading file for hashing",
			filePath,
			"Failed to read file content for hash calculation",
			err,
		)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// HasChanged checks if a secret has changed since last deployment
func (hs *HashStore) hasChanged(filePath string) (bool, error) {
	// Calculate current hash
	currentHash, err := hs.calculateHash(filePath)
	if err != nil {
		return false, err
	}

	// Get file info for modification time
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return false, errors.FileOperationError(
			"Getting file info",
			filePath,
			"Failed to get file information",
			err,
		)
	}

	// Check if we have a previous hash
	previousHash, exists := hs.Hashes[filePath]
	if !exists {
		// First time seeing this file - it's "changed"
		hs.Hashes[filePath] = SecretHash{
			Path:         filePath,
			Hash:         currentHash,
			LastModified: fileInfo.ModTime(),
		}
		return true, nil
	}

	// Compare hashes
	if previousHash.Hash != currentHash {
		// Content changed - update stored hash
		hs.Hashes[filePath] = SecretHash{
			Path:         filePath,
			Hash:         currentHash,
			LastModified: fileInfo.ModTime(),
		}
		return true, nil
	}

	// No change detected
	return false, nil
}

// ExtractServiceActions extracts service actions from secret configuration
func (m *Manager) ExtractServiceActions(secret config.Secret, secretName string) ([]ServiceAction, error) {
	if secret.Services == nil {
		return nil, nil
	}

	var actions []ServiceAction

	switch services := secret.Services.(type) {
	case []interface{}:
		// Simple list of service names
		for _, svc := range services {
			if serviceName, ok := svc.(string); ok {
				actions = append(actions, ServiceAction{
					Name:    serviceName,
					Restart: m.config.RestartOnChange,
					After:   []string{"opnix-secrets.service"},
				})
			}
		}

	case map[string]interface{}:
		// Advanced service configuration
		for serviceName, svcConfig := range services {
			action := ServiceAction{
				Name:    serviceName,
				Restart: m.config.RestartOnChange,
				After:   []string{"opnix-secrets.service"},
			}

			// Parse service configuration
			if configMap, ok := svcConfig.(map[string]interface{}); ok {
				if restart, ok := configMap["restart"].(bool); ok {
					action.Restart = restart
				}
				if signal, ok := configMap["signal"].(string); ok {
					action.Signal = signal
				}
				if after, ok := configMap["after"].([]interface{}); ok {
					var afterServices []string
					for _, a := range after {
						if afterService, ok := a.(string); ok {
							afterServices = append(afterServices, afterService)
						}
					}
					action.After = afterServices
				}
			}

			actions = append(actions, action)
		}

	default:
		return nil, errors.ConfigError(
			fmt.Sprintf("Parsing services for secret %s", secretName),
			"Services field must be an array of strings or object with service configurations",
			nil,
		)
	}

	return actions, nil
}

// ProcessSecretChanges processes secrets and determines which services need restart
func (m *Manager) ProcessSecretChanges(secrets []config.Secret, secretPaths map[string]string) error {
	if !m.config.Enable {
		return nil
	}

	var changedSecrets []string
	var allServiceActions []ServiceAction

	// Check each secret for changes
	for i, secret := range secrets {
		secretName := fmt.Sprintf("secret[%d]:%s", i, secret.Path)

		// Get the actual file path for this secret
		var secretPath string
		if secret.Path != "" {
			if filepath.IsAbs(secret.Path) {
				secretPath = secret.Path
			} else {
				// This would need to be calculated based on the path resolution logic
				// For now, assume it's provided in secretPaths
				if path, exists := secretPaths[secretName]; exists {
					secretPath = path
				} else {
					continue // Skip if we can't determine the path
				}
			}
		}

		// Check if change detection is enabled
		hasChanged := true // Default to always changed if detection disabled
		if m.config.ChangeDetection.Enable && m.hashStore != nil {
			var err error
			hasChanged, err = m.hashStore.hasChanged(secretPath)
			if err != nil {
				if m.config.ErrorHandling.ContinueOnError {
					fmt.Fprintf(os.Stderr, "WARNING: Failed to check changes for %s: %v\n", secretName, err)
					continue
				}
				return err
			}
		}

		if hasChanged {
			changedSecrets = append(changedSecrets, secretName)

			// Extract service actions for this secret
			actions, err := m.ExtractServiceActions(secret, secretName)
			if err != nil {
				if m.config.ErrorHandling.ContinueOnError {
					fmt.Fprintf(os.Stderr, "WARNING: Failed to extract service actions for %s: %v\n", secretName, err)
					continue
				}
				return err
			}

			allServiceActions = append(allServiceActions, actions...)
		}
	}

	// Save hash store if we have changes and change detection is enabled
	if len(changedSecrets) > 0 && m.config.ChangeDetection.Enable && m.hashStore != nil {
		if err := m.hashStore.save(); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: Failed to save hash store: %v\n", err)
		}
	}

	// Process service actions if we have changes
	if len(allServiceActions) > 0 {
		fmt.Printf("INFO: Processing %d changed secrets: %v\n", len(changedSecrets), changedSecrets)
		return m.processServiceActions(allServiceActions)
	}

	fmt.Printf("INFO: No secret changes detected, skipping service restarts\n")
	return nil
}

// processServiceActions executes the required service actions
func (m *Manager) processServiceActions(actions []ServiceAction) error {
	// Group actions by service to avoid duplicate operations
	serviceActions := make(map[string]ServiceAction)
	for _, action := range actions {
		// If we already have an action for this service, prefer restart over reload
		if existing, exists := serviceActions[action.Name]; exists {
			if action.Restart && !existing.Restart {
				serviceActions[action.Name] = action
			}
		} else {
			serviceActions[action.Name] = action
		}
	}

	// Execute actions with retry logic
	var failures []string
	for serviceName, action := range serviceActions {
		if err := m.executeServiceAction(action); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", serviceName, err))

			if !m.config.ErrorHandling.ContinueOnError {
				return errors.ServiceError(
					fmt.Sprintf("Executing service action for %s", serviceName),
					serviceName,
					"restart/reload",
					err,
				)
			}
		}
	}

	if len(failures) > 0 {
		fmt.Fprintf(os.Stderr, "WARNING: Some service actions failed: %v\n", failures)
	}

	return nil
}

// executeServiceAction executes a single service action with retry logic
func (m *Manager) executeServiceAction(action ServiceAction) error {
	var cmd string
	var args []string

	if action.Signal != "" {
		// Send custom signal
		cmd = "kill"
		args = []string{"-" + action.Signal, fmt.Sprintf("$(systemctl show -p MainPID --value %s)", action.Name)}
		fmt.Printf("INFO: Sending %s signal to service %s\n", action.Signal, action.Name)
	} else if action.Restart {
		// Restart service
		cmd = m.systemctl
		args = []string{"restart", action.Name}
		fmt.Printf("INFO: Restarting service %s\n", action.Name)
	} else {
		// Reload service
		cmd = m.systemctl
		args = []string{"reload", action.Name}
		fmt.Printf("INFO: Reloading service %s\n", action.Name)
	}

	// Execute with retry logic
	var lastErr error
	for attempt := 0; attempt < m.config.ErrorHandling.MaxRetries; attempt++ {
		if attempt > 0 {
			fmt.Printf("INFO: Retrying service action for %s (attempt %d/%d)\n",
				action.Name, attempt+1, m.config.ErrorHandling.MaxRetries)
			time.Sleep(time.Duration(attempt) * time.Second)
		}

		if m.dryRun {
			fmt.Printf("DRY-RUN: Would execute: %s %s\n", cmd, strings.Join(args, " "))
			return nil
		}

		execCmd := exec.Command(cmd, args...)
		output, err := execCmd.CombinedOutput()
		if err != nil {
			lastErr = fmt.Errorf("command failed: %v, output: %s", err, string(output))
			continue
		}

		// Success
		fmt.Printf("INFO: Successfully executed service action for %s\n", action.Name)
		return nil
	}

	return lastErr
}

// SetDryRun enables dry-run mode for testing
func (m *Manager) SetDryRun(dryRun bool) {
	m.dryRun = dryRun
}

// IsServiceRunning checks if a systemd service is currently running
func (m *Manager) IsServiceRunning(serviceName string) (bool, error) {
	cmd := exec.Command(m.systemctl, "is-active", "--quiet", serviceName)
	err := cmd.Run()

	if err == nil {
		return true, nil
	}

	// Check if it's an exit status error (service not running) vs other error
	if exitError, ok := err.(*exec.ExitError); ok {
		// systemctl is-active returns exit code 3 for inactive services
		if exitError.ExitCode() == 3 {
			return false, nil
		}
	}

	// Other error occurred
	return false, errors.ServiceError(
		"Checking service status",
		serviceName,
		"is-active",
		err,
	)
}

// ValidateServices checks that all configured services exist and are valid
func (m *Manager) ValidateServices(services []string) error {
	for _, serviceName := range services {
		// Check if service unit exists
		cmd := exec.Command(m.systemctl, "cat", serviceName)
		if err := cmd.Run(); err != nil {
			return errors.ServiceError(
				"Validating service configuration",
				serviceName,
				"cat",
				fmt.Errorf("service unit not found or not accessible"),
			)
		}
	}

	return nil
}
