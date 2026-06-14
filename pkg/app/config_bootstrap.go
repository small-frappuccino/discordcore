package app

import (
	"fmt"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

const (
	databaseDriverEnv              = "DISCORDCORE_DATABASE_DRIVER"
	databaseURLEnv                 = "DISCORDCORE_DATABASE_URL"
	databaseMaxOpenConnsEnv        = "DISCORDCORE_DATABASE_MAX_OPEN_CONNS"
	databaseMaxIdleConnsEnv        = "DISCORDCORE_DATABASE_MAX_IDLE_CONNS"
	databaseConnMaxLifetimeSecsEnv = "DISCORDCORE_DATABASE_CONN_MAX_LIFETIME_SECS"
	databaseConnMaxIdleTimeSecsEnv = "DISCORDCORE_DATABASE_CONN_MAX_IDLE_TIME_SECS"
	databasePingTimeoutMSEnv       = "DISCORDCORE_DATABASE_PING_TIMEOUT_MS"
)

type resolvedDatabaseBootstrap struct {
	Config files.DatabaseRuntimeConfig
	Source string
}

func resolveDatabaseBootstrap() (resolvedDatabaseBootstrap, error) {
	if cfg, ok := databaseBootstrapFromEnv(); ok {
		return resolvedDatabaseBootstrap{
			Config: cfg,
			Source: "env",
		}, nil
	}
	return resolvedDatabaseBootstrap{}, fmt.Errorf(
		"postgres bootstrap config unavailable: set %s before startup",
		databaseURLEnv,
	)
}

func databaseBootstrapFromEnv() (files.DatabaseRuntimeConfig, bool) {
	url := files.EnvString(databaseURLEnv, "")
	if url == "" {
		return files.DatabaseRuntimeConfig{}, false
	}

	driver := files.EnvString(databaseDriverEnv, "postgres")
	return files.DatabaseRuntimeConfig{
		Driver:              driver,
		DatabaseURL:         url,
		MaxOpenConns:        int(files.EnvInt64(databaseMaxOpenConnsEnv, 20)),
		MaxIdleConns:        int(files.EnvInt64(databaseMaxIdleConnsEnv, 10)),
		ConnMaxLifetimeSecs: int(files.EnvInt64(databaseConnMaxLifetimeSecsEnv, 1800)),
		ConnMaxIdleTimeSecs: int(files.EnvInt64(databaseConnMaxIdleTimeSecsEnv, 300)),
		PingTimeoutMS:       int(files.EnvInt64(databasePingTimeoutMSEnv, 5000)),
	}, true
}

func syncBootstrapDatabaseConfig(configManager *files.ConfigManager, cfg files.DatabaseRuntimeConfig) error {
	if configManager == nil {
		return fmt.Errorf("config manager is nil")
	}

	current := configManager.SnapshotConfig().RuntimeConfig.Database
	if current == cfg {
		return nil
	}

	_, err := configManager.UpdateRuntimeConfig(func(rc *files.RuntimeConfig) error {
		rc.Database = cfg
		return nil
	})
	if err != nil {
		return fmt.Errorf("persist runtime database config: %w", err)
	}
	return nil
}
