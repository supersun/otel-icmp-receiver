package icmpreceiver

import (
	"context"
	"testing"

	"go.uber.org/goleak"

	"github.com/supersun/otel-icmp-receiver/internal/metadata"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestCreateMetrics(t *testing.T) {
	t.Run(
		"Nil config gives error", func(t *testing.T) {
			recv, err := createMetricsReceiver(
				context.Background(),
				receivertest.NewNopSettings(metadata.Type),
				nil,
				&consumertest.MetricsSink{},
			)

			require.Nil(t, recv)
			require.Error(t, err)
			require.ErrorIs(t, err, errConfigNotPingReceiver)
		},
	)

	t.Run(
		"Metrics receiver is created with default config", func(t *testing.T) {
			recv, err := createMetricsReceiver(
				context.Background(),
				receivertest.NewNopSettings(metadata.Type),
				createDefaultConfig(),
				&consumertest.MetricsSink{},
			)

			require.NoError(t, err)
			require.NotNil(t, recv)

			// The receiver must be able to shutdown cleanly without a Start call.
			err = recv.Shutdown(context.Background())
			require.NoError(t, err)
		},
	)
}
