package icmpreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/zap"
)

const (
	collectionInterval = time.Duration(1 * time.Minute)
	defaultPingTimeout = 1 * time.Second
)

var (
	testLogger, _ = zap.NewDevelopment()

	testControllerCfg = scraperhelper.ControllerConfig{
		CollectionInterval: collectionInterval,
		InitialDelay:       5 * time.Second,
	}
	testSettings = receiver.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: testLogger,
		},
	}
)

func TestSuccessfulPingScrape(t *testing.T) {
	// Setup config
	cfg := &Config{
		ControllerConfig:   testControllerCfg,
		Targets:            []Target{{Target: "8.8.8.8"}}, // Google's public DNS
		DefaultPingCount:   4,
		DefaultPingTimeout: defaultPingTimeout,
	}

	// Create the scraper
	pingScraper, err := newPingScraper(cfg, testSettings)
	assert.NoError(t, err)

	// Simulate a scrape
	metrics, err := pingScraper.Scrape(context.Background())

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, metrics)

	resourceMetrics := metrics.ResourceMetrics()
	assert.NotNil(t, resourceMetrics)
	assert.Equal(t, 1, resourceMetrics.Len())

	// Verify that the scrape contains the expected metrics
	scopeMetrics := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	assert.Equal(
		t, 6, scopeMetrics.Len(),
	) // We expect 6 metrics: rtt, rtt.min, rtt.max, rtt.avg, rtt.stddev, loss.ratio

	// Verify that one of the metrics has data
	rttMetric := scopeMetrics.At(0)
	assert.Equal(t, "ping.rtt", rttMetric.Name())
	assert.Equal(t, "ms", rttMetric.Unit())

	// Check if data points exist before accessing them
	rttDataPoints := rttMetric.Gauge().DataPoints()
	if rttDataPoints.Len() > 0 {
		// Verify that the data points for rtt contain expected values
		rttDataPoint := rttDataPoints.At(0)
		assert.Greater(t, rttDataPoint.DoubleValue(), 0.0)
	} else {
		// If ping.rtt has no data points (e.g., ICMP blocked, network unavailable, or all packets timed out),
		// check that stats metrics still have data points (they should be added even if no packets were received)
		// The stats metrics should have at least 1 data point per target
		lossRatioMetric := scopeMetrics.At(5) // loss.ratio is the 6th metric (index 5)
		lossRatioDataPoints := lossRatioMetric.Gauge().DataPoints()
		assert.Greater(
			t, lossRatioDataPoints.Len(), 0,
			"Stats metrics should have data points even if ping.rtt has no packets (ping was attempted)",
		)
		// Verify the stats metric has valid data
		lossRatioDataPoint := lossRatioDataPoints.At(0)
		assert.GreaterOrEqual(t, lossRatioDataPoint.DoubleValue(), 0.0, "Loss ratio should be >= 0")
		t.Logf(
			"ping.rtt has no data points (likely ICMP blocked, network issue, or all packets timed out), but stats metrics exist with %d data points",
			lossRatioDataPoints.Len(),
		)
	}
}

func TestPingScrapeWithDNSError(t *testing.T) {
	// config with an invalid target (unresolvable DNS)
	cfg := &Config{
		ControllerConfig:   testControllerCfg,
		Targets:            []Target{{Target: "invalid.target.com"}},
		DefaultPingCount:   4,
		DefaultPingTimeout: defaultPingTimeout,
	}

	pingScraper, err := newPingScraper(cfg, testSettings)
	assert.NoError(t, err)

	metrics, err := pingScraper.Scrape(context.Background())

	assert.NoError(t, err)    // No error should be returned, just a warning
	assert.NotNil(t, metrics) // Metrics should still be returned, though they will be empty

	resourceMetrics := metrics.ResourceMetrics()
	assert.NotNil(t, resourceMetrics)
	assert.Equal(t, 1, resourceMetrics.Len()) // We expect exactly 1 ResourceMetrics (even if empty)

	scopeMetrics := resourceMetrics.At(0).ScopeMetrics().At(0).Metrics()
	assert.Equal(
		t, 6, scopeMetrics.Len(),
	) // Expecting 6 metrics (rtt, rtt.min, rtt.max, rtt.avg, rtt.stddev, loss.ratio)

	// Check that each of the metrics has no data points
	for i := 0; i < scopeMetrics.Len(); i++ {
		metric := scopeMetrics.At(i)
		dataPoints := metric.Gauge().DataPoints()
		// Ensure that the metric has no data points due to DNS failure
		assert.Equal(t, 0, dataPoints.Len(), "No data points should exist for any metric due to DNS failure")
	}
}

func TestPingScrapeWithTimeout(t *testing.T) {
	// config with a very short ping timeout
	cfg := &Config{
		ControllerConfig:   testControllerCfg,
		Targets:            []Target{{Target: "8.8.8.8"}}, // Google's public DNS
		DefaultPingCount:   4,
		DefaultPingTimeout: 1 * time.Nanosecond, // Very short timeout
	}

	pingScraper, err := newPingScraper(cfg, testSettings)
	assert.NoError(t, err)

	metrics, err := pingScraper.Scrape(context.Background())

	// Assertions
	assert.NoError(t, err)    // No error should be returned
	assert.NotNil(t, metrics) // Metrics should still be returned, though they will be empty

	// Check ResourceMetrics
	resourceMetrics := metrics.ResourceMetrics()
	assert.NotNil(t, resourceMetrics)
	assert.Equal(t, 1, resourceMetrics.Len()) // Expecting 1 ResourceMetrics

	// Check that we have exactly 6 metrics (rtt, rtt.min, rtt.max, rtt.avg, rtt.stddev, loss.ratio)
	scopeMetrics := resourceMetrics.At(0).ScopeMetrics().At(0).Metrics()
	assert.Equal(t, 6, scopeMetrics.Len())

	// Ensure that there are no data points for each metric (since ping timed out)
	for i := 0; i < scopeMetrics.Len(); i++ {
		metric := scopeMetrics.At(i)
		dataPoints := metric.Gauge().DataPoints()
		if metric.Name() != "ping.rtt" {
			assert.Equal(t, 1, dataPoints.Len(), "Empty data points should exist due to the timeout error")
		} else {
			assert.Equal(t, 0, dataPoints.Len(), "No data points should exist for rtt metric due to ping timeout")
		}
	}
}

func TestPingScrapeWithMultipleTargets(t *testing.T) {
	// Setup config with multiple valid targets
	cfg := &Config{
		ControllerConfig:   testControllerCfg,
		Targets:            []Target{{Target: "8.8.8.8"}, {Target: "1.1.1.1"}}, // Google's and Cloudflare's public DNS
		DefaultPingCount:   4,
		DefaultPingTimeout: defaultPingTimeout,
	}

	pingScraper, err := newPingScraper(cfg, testSettings)
	assert.NoError(t, err)

	metrics, err := pingScraper.Scrape(context.Background())

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, metrics)

	resourceMetrics := metrics.ResourceMetrics()
	assert.NotNil(t, resourceMetrics)
	assert.Equal(t, 1, resourceMetrics.Len()) // Only one resource metric should be returned

	// Verify that the scrape contains the expected metrics
	scopeMetrics := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	assert.Equal(
		t, 6, scopeMetrics.Len(),
	) // We expect 6 metrics: rtt, rtt.min, rtt.max, rtt.avg, rtt.stddev, loss.ratio

	// Verify that there are data points for both targets (if ping succeeded)
	// Note: If ICMP is blocked or network unavailable, there may be 0 data points
	rttMetric := scopeMetrics.At(0)
	rttDataPoints := rttMetric.Gauge().DataPoints()
	if rttDataPoints.Len() > 0 {
		// If we have data points, verify we have the expected number (2 targets = 2 data points if both succeed)
		assert.GreaterOrEqual(t, rttDataPoints.Len(), 1, "Should have at least 1 data point if ping succeeded")
		// Check that stats metrics have data points for both targets
		lossRatioMetric := scopeMetrics.At(5)
		assert.Equal(
			t, 2, lossRatioMetric.Gauge().DataPoints().Len(), "Stats metrics should have data points for both targets",
		)
	} else {
		// If ping.rtt has no data points, stats metrics should still have data
		lossRatioMetric := scopeMetrics.At(5)
		assert.Greater(
			t, lossRatioMetric.Gauge().DataPoints().Len(), 0,
			"Stats metrics should have data points even if ping.rtt has none",
		)
		t.Logf(
			"ping.rtt has no data points, but stats metrics have %d data points",
			lossRatioMetric.Gauge().DataPoints().Len(),
		)
	}
}
