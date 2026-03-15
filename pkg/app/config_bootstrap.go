package app

import (
	"fmt"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/util"
)

const (
	databaseDriverEnv              = "ALICE_DATABASE_DRIVER"
	databaseURLEnv                 = "ALICE_DATABASE_URL"
	databaseMaxOpenConnsEnv        = "ALICE_DATABASE_MAX_OPEN_CONNS"
	databaseMaxIdleConnsEnv        = "ALICE_DATABASE_MAX_IDLE_CONNS"
	databaseConnMaxLifetimeSecsEnv = "ALICE_DATABASE_CONN_MAX_LIFETIME_SECS"
	databaseConnMaxIdleTimeSecsEnv = "ALICE_DATABASE_CONN_MAX_IDLE_TIME_SECS"
	databasePingTimeoutMSEnv       = "ALICE_DATABASE_PING_TIMEOUT_MS"
)

type resolvedDatabaseBootstrap struct {
	Config             files.DatabaseRuntimeConfig
	Source             string
	LegacySettingsPath string
}

func resolveDatabaseBootstrap() (resolvedDatabaseBootstrap, error) {
	if cfg, ok := databaseBootstrapFromEnv(); ok {
		return resolvedDatabaseBootstrap{
			Config:             cfg,
			Source:             "env",
			LegacySettingsPath: util.GetSettingsFilePath(),
		}, nil
	}
	return resolvedDatabaseBootstrap{}, fmt.Errorf(
		"postgres bootstrap config unavailable: set %s before startup; legacy settings.json is only used for one-time config migration after postgres is reachable",
		databaseURLEnv,
	)
}

func databaseBootstrapFromEnv() (files.DatabaseRuntimeConfig, bool) {
	url := util.EnvString(databaseURLEnv, "")
	if url == "" {
		return files.DatabaseRuntimeConfig{}, false
	}

	driver := util.EnvString(databaseDriverEnv, "postgres")
	return files.DatabaseRuntimeConfig{
		Driver:              driver,
		DatabaseURL:         url,
		MaxOpenConns:        int(util.EnvInt64(databaseMaxOpenConnsEnv, 20)),
		MaxIdleConns:        int(util.EnvInt64(databaseMaxIdleConnsEnv, 10)),
		ConnMaxLifetimeSecs: int(util.EnvInt64(databaseConnMaxLifetimeSecsEnv, 1800)),
		ConnMaxIdleTimeSecs: int(util.EnvInt64(databaseConnMaxIdleTimeSecsEnv, 300)),
		PingTimeoutMS:       int(util.EnvInt64(databasePingTimeoutMSEnv, 5000)),
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
