package fs

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func GetUserAppDataDir(appName string) (string, error) {
	var base string

	switch runtime.GOOS {
	case "windows":
		base = os.Getenv("AppData") // e.g. C:\Users\user\AppData\Roaming
	case "darwin":
		base = filepath.Join(os.Getenv("HOME"), "Library", "Application Support")
	default: // Linux and others
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}

	if base == "" {
		return "", fmt.Errorf("could not determine base config path")
	}

	appDataPath := filepath.Join(base, appName)
	err := os.MkdirAll(appDataPath, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create app data dir: %w", err)
	}

	return appDataPath, nil
}

func GetProgramDataDir(appName string) (string, error) {
	var baseDir string

	switch runtime.GOOS {
	case "windows":
		baseDir = os.Getenv("PROGRAMDATA")
		if baseDir == "" {
			baseDir = `C:\ProgramData`
		}
	case "darwin":
		baseDir = "/Library/Application Support"
	default:
		baseDir = "/var/lib"
	}

	fullPath := filepath.Join(baseDir, appName)

	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return "", err
	}

	return fullPath, nil
}

func EnsureDir(path string) error {
	err := os.MkdirAll(path, 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return nil

}
