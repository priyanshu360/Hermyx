package engine

import (
	"hermyx/pkg/models"
	"hermyx/pkg/utils/fs"
	"hermyx/pkg/utils/hash"
	"hermyx/pkg/utils/system"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

func InitConfig(configPath string) error {
	appData, err := fs.GetUserAppDataDir("hermyx")
	if err != nil {
		return err
	}

	hash := hash.HashString(configPath)
	storageDir := filepath.Join(appData, hash)

	freePort, err := system.GetFreePort()
	if err != nil {
		return err
	}

	defaultConfig := &models.HermyxConfig{
		Log: &models.LogConfig{
			ToFile:   true,
			FilePath: filepath.Join(storageDir, "hermyx.log"),
			ToStdout: true,
			Prefix:   "[Hermyx]",
			Flags:    0,
		},
		Server: &models.ServerConfig{
			Port: uint16(freePort),
		},
		Storage: &models.StorageConfig{
			Path: storageDir,
		},
		Cache: &models.CacheConfig{
			Enabled:        true,
			Type:           "memory",
			Ttl:            5 * time.Minute,
			Capacity:       1000,
			MaxContentSize: 1048576,
			KeyConfig: &models.CacheKeyConfig{
				Type:           []string{"path", "method", "query", "header"},
				ExcludeMethods: []string{"post", "put"},
				Headers: []*models.HeaderCacheKeyConfig{
					{
						"x-device-id",
					},
				},
			},
		},
		Routes: []models.RouteConfig{
			{
				Name:    "example-route",
				Path:    "^/api/example",
				Target:  "localhost:3000",
				Include: []string{".*"},
				Exclude: []string{},
				Cache: &models.CacheConfig{
					Enabled: true,
					Ttl:     2 * time.Minute,
					KeyConfig: &models.CacheKeyConfig{
						Type:           []string{"path", "query"},
						ExcludeMethods: []string{"post"},
					},
				},
			},
		},
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	f, err := os.Create(configPath)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := yaml.NewEncoder(f)
	enc.SetIndent(2)
	return enc.Encode(defaultConfig)
}
