package icmpreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/supersun/otel-icmp-receiver/internal/metadata"
)

func TestDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg, "failed to create default config")
	require.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestLoadConfig(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[metadata.Type] = factory

	cfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Len(t, cfg.Receivers, 2)

	r0 := cfg.Receivers[component.NewID(metadata.Type)]
	defaultICMPCheckReceiver := factory.CreateDefaultConfig()

	defaultICMPCheckReceiver.(*Config).ControllerConfig.CollectionInterval = 10 * time.Second

	defaultICMPCheckReceiver.(*Config).DefaultPingCount = 3
	defaultICMPCheckReceiver.(*Config).DefaultPingTimeout = 5 * time.Second

	targets := testDataConfigYamlTargets()

	defaultICMPCheckReceiver.(*Config).Targets = targets

	assert.Equal(t, defaultICMPCheckReceiver, r0)

	r1 := cfg.Receivers[component.NewIDWithName(metadata.Type, "custom-5s")].(*Config)
	assert.Equal(t, testDataConfigYaml5s(), r1)
}

func TestLoadInvalidConfig_NoTargets(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[metadata.Type] = factory
	_, err = otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config-invalid-no-targets.yaml"), factories)
	t.Log(err)

	require.ErrorContains(t, err, "\"collection_interval\": requires positive value")
	require.ErrorContains(t, err, "\"default_ping_count\": cannot be lesser than 3")
	require.ErrorContains(t, err, "\"default_ping_timeout\": cannot be lesser than 5s")
	require.ErrorContains(t, err, "\"targets\": cannot be empty or nil")
}

func testDataConfigYamlTargets() []Target {
	targets := []Target{
		{
			Target:      "www.amazon.de",
			PingCount:   func(v int) *int { return &v }(4),
			PingTimeout: func(v time.Duration) *time.Duration { d := 5 * time.Second; return &d }(5 * time.Second),
		},
		{
			Target: "www.amazon.com",
		},
		{
			Target: "www.doesnot123exiiiiist.coom",
		},
		{
			Target: "api.amazon.com",
		},
		{
			Target:      "api.amazon.de",
			PingTimeout: func(v time.Duration) *time.Duration { d := 2 * time.Second; return &d }(5 * time.Second),
		},
		{
			Target: "8.8.8.8",
		},
	}
	return targets
}

func testDataConfigYaml5s() *Config {
	expectedConfig := &Config{
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: 5 * time.Second,
			InitialDelay:       1 * time.Second,
		},
		DefaultPingCount:   4,
		DefaultPingTimeout: 5 * time.Second,
		Tag:                "fake-custom-5s-tag",
		Targets: []Target{
			{
				Target: "www.bbc.com",
			},
		},
	}
	return expectedConfig
}

func TestLoadInvalidConfig_DuplicateTargets(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[metadata.Type] = factory
	_, err = otelcoltest.LoadConfigAndValidate(
		filepath.Join("testdata", "config-invalid-duplicate-targets.yaml"), factories,
	)
	t.Log(err)

	require.NotNil(t, err)

	require.ErrorContains(t, err, "value **\"localhost1\"** is duplicated")
	require.ErrorContains(t, err, "value **\"localhost5\"** is duplicated")
	require.ErrorContains(t, err, "value **\"localhost6\"** is duplicated")
	require.ErrorContains(t, err, "value **\"localhost7\"** is duplicated")
}

func TestLoadInvalidConfig_InvalidTags(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[metadata.Type] = factory
	_, err = otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config-invalid-tags.yaml"), factories)
	t.Log(err)

	require.NotNil(t, err)

	require.ErrorContains(t, err, "cannot contain spaces")
}
