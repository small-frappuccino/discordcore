package persistence

import (
	"fmt"
	"strings"
	"time"
)

// Config defines database connectivity and pool options.
type Config struct {
	Driver              string
	DatabaseURL         string
	MaxOpenConns        int
	MaxIdleConns        int
	ConnMaxLifetimeSecs int
	ConnMaxIdleTimeSecs int
	PingTimeoutMS       int
}

func (c Config) Normalized() Config {
	out := c
	out.Driver = strings.ToLower(strings.TrimSpace(out.Driver))
	out.DatabaseURL = strings.TrimSpace(out.DatabaseURL)
	if out.MaxOpenConns <= 0 {
		out.MaxOpenConns = 20
	}
	if out.MaxIdleConns < 0 {
		out.MaxIdleConns = 0
	}
	if out.ConnMaxLifetimeSecs <= 0 {
		out.ConnMaxLifetimeSecs = int((30 * time.Minute).Seconds())
	}
	if out.ConnMaxIdleTimeSecs <= 0 {
		out.ConnMaxIdleTimeSecs = int((5 * time.Minute).Seconds())
	}
	if out.PingTimeoutMS <= 0 {
		out.PingTimeoutMS = int((5 * time.Second).Milliseconds())
	}
	return out
}

func (c Config) Validate() error {
	n := c.Normalized()
	if n.Driver == "" {
		return fmt.Errorf("database driver is required (expected: postgres)")
	}
	if n.Driver != "postgres" {
		return fmt.Errorf("unsupported database driver %q (supported: postgres)", n.Driver)
	}
	if n.DatabaseURL == "" {
		return fmt.Errorf("database_url is required in runtime_config.database")
	}
	return nil
}
