package icmpreceiver

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/supersun/otel-icmp-receiver/internal/metadata"
)

var errConfigNotPingReceiver = fmt.Errorf("config is not valid for the '%s' receiver", metadata.Type)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	cfg := scraperhelper.NewDefaultControllerConfig()

	return &Config{
		ControllerConfig: cfg,
		Targets:          []Target{},
		Tag:              TagNotSet,
	}
}

func createMetricsReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	receiverCfg, ok := cfg.(*Config)
	if !ok {
		return nil, errConfigNotPingReceiver
	}
	set.Logger.Info("about creating new icmp check receiver - newPingScraper")

	opts := []scraperhelper.ControllerOption{}

	icmpScraper, err := newPingScraper(receiverCfg, set)
	if err != nil {
		return nil, err
	}

	scp, err := scraper.NewMetrics(icmpScraper.Scrape)
	if err != nil {
		return nil, err
	}

	opts = append(opts, scraperhelper.AddScraper(metadata.Type, scp))

	set.Logger.Info("about creating new icmp check receiver - scraperhelper.NewMetricsController")

	return scraperhelper.NewMetricsController(&receiverCfg.ControllerConfig, set, nextConsumer, opts...)
}
