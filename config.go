package icmpreceiver

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"
)

var (
	mu                     sync.Mutex              // mu is used to synchronize access to globalTargets
	globalTargets          = make(map[string]bool) // globalTargets is used to store all targets to check for duplicates
	errNonPositiveInterval = errors.New("requires positive value")
)

type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	Targets                        []Target      `mapstructure:"targets"`
	DefaultPingCount               int           `mapstructure:"default_ping_count"`
	DefaultPingTimeout             time.Duration `mapstructure:"default_ping_timeout"`
	Tag                            string        `mapstructure:"tag"`
}

type Target struct {
	Target string `mapstructure:"target"`

	PingCount   *int           `mapstructure:"ping_count"`
	PingTimeout *time.Duration `mapstructure:"ping_timeout"`
}

func (c *Config) Validate() (errs error) {
	if c.CollectionInterval <= 0 {
		errs = multierr.Append(errs, fmt.Errorf(`"collection_interval": %w`, errNonPositiveInterval))
	}
	if c.Tag == "" {
		c.Tag = TagNotSet
	} else if containsSpaces(c.Tag) {
		errs = multierr.Append(errs, fmt.Errorf(`[%s] %s`, c.Tag, `"tag": cannot contain spaces`))
	}

	if c.Timeout < 0 {
		errs = multierr.Append(errs, fmt.Errorf(`"timeout": %w`, errNonPositiveInterval))
	}

	if c.DefaultPingCount < 3 {
		errs = multierr.Append(errs, fmt.Errorf(`"default_ping_count": %s`, "cannot be lesser than 3"))
	}
	if c.DefaultPingTimeout < 5*time.Second {
		errs = multierr.Append(errs, fmt.Errorf(`"default_ping_timeout": %s`, "cannot be lesser than 5s"))
	}

	if len(c.Targets) == 0 {
		errs = multierr.Append(errs, fmt.Errorf(`"targets": %s`, "cannot be empty or nil"))
	}

	for i, target := range c.Targets {
		if target.PingCount != nil && *target.PingCount < 1 {
			errs = multierr.Append(errs, fmt.Errorf("target #%d has invalid ping_count %d", i, *target.PingCount))
		}
		if target.PingTimeout != nil && *target.PingTimeout <= 1*time.Second {
			errs = multierr.Append(errs, fmt.Errorf("target #%d has invalid ping_timeout %v", i, *target.PingTimeout))
		}

		// Check for duplicates
		mu.Lock()
		if globalTargets[target.Target] {
			errs = multierr.Append(errs, fmt.Errorf("target #%d with value **%q** is duplicated", i+1, target.Target))
		} else {
			globalTargets[target.Target] = true
		}
		mu.Unlock()
	}

	return
}

func containsSpaces(s string) bool {
	for _, r := range s {
		if r == ' ' {
			return true
		}
	}
	return false
}
