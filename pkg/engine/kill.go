package engine

import (
	"fmt"
	"hermyx/pkg/models"
	"hermyx/pkg/utils/fs"
	"hermyx/pkg/utils/hash"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"gopkg.in/yaml.v3"
)

func KillHermyx(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var config models.HermyxConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	if config.Storage == nil || config.Storage.Path == "" {
		storageRoot, err := fs.GetUserAppDataDir("hermyx")
		if err != nil {
			return fmt.Errorf("failed to determine app data dir: %w", err)
		}
		absConfigPath, err := filepath.Abs(configPath)
		if err != nil {
			return fmt.Errorf("failed to resolve absolute config path: %w", err)
		}
		config.Storage = &models.StorageConfig{filepath.Join(storageRoot, hash.HashString(absConfigPath))}
	}

	pidPath := filepath.Join(config.Storage.Path, "hermyx.pid")
	pidData, err := os.ReadFile(pidPath)
	if err != nil {
		return fmt.Errorf("failed to read PID file: %w", err)
	}

	pid, err := strconv.Atoi(string(pidData))
	if err != nil {
		return fmt.Errorf("invalid PID content in %s: %w", pidPath, err)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process with PID %d: %w", pid, err)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM to process %d: %w", pid, err)
	}

	return nil
}
