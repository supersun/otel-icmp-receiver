package icmpreceiver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"go.opentelemetry.io/collector/receiver"

	probing "github.com/prometheus-community/pro-bing"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

const (
	AttrPeerIp   = "net.peer.ip"
	AttrPeerName = "net.peer.name"
	AttrTag      = "tag"
	TagNotSet    = "NA"
)

type packet struct {
	Timestamp time.Time
	*probing.Packet
}

type pingResult struct {
	Packets        []*packet
	Stats          *probing.Statistics
	StatsTimestamp time.Time
	tag            string
}

type pingScraper struct {
	logger             *zap.Logger
	collectionInterval time.Duration

	targets            []Target
	defaultPingCount   int
	defaultPingTimeout time.Duration
	tag                string
}

func newPingScraper(
	receiverCfg *Config,
	settings receiver.Settings,
) (*pingScraper, error) {
	return &pingScraper{
		logger:             settings.Logger,
		collectionInterval: receiverCfg.CollectionInterval,

		targets:            receiverCfg.Targets,
		defaultPingCount:   receiverCfg.DefaultPingCount,
		defaultPingTimeout: receiverCfg.DefaultPingTimeout,
		tag:                receiverCfg.Tag,
	}, nil
}

func (s *pingScraper) Scrape(_ context.Context) (pmetric.Metrics, error) {
	metrics := pmetric.NewMetrics()
	scopeMetrics := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()

	rttMetric := scopeMetrics.AppendEmpty()
	rttMetric.SetName("ping.rtt")
	rttMetric.SetUnit("ms")
	rttMetricDataPoints := rttMetric.SetEmptyGauge().DataPoints()

	minRttMetric := scopeMetrics.AppendEmpty()
	minRttMetric.SetName("ping.rtt.min")
	minRttMetric.SetUnit("ms")
	minRttMetricDataPoints := minRttMetric.SetEmptyGauge().DataPoints()

	maxRttMetric := scopeMetrics.AppendEmpty()
	maxRttMetric.SetName("ping.rtt.max")
	maxRttMetric.SetUnit("ms")
	maxRttMetricDataPoints := maxRttMetric.SetEmptyGauge().DataPoints()

	avgRttMetric := scopeMetrics.AppendEmpty()
	avgRttMetric.SetName("ping.rtt.avg")
	avgRttMetric.SetUnit("ms")
	avgRttMetricDataPoints := avgRttMetric.SetEmptyGauge().DataPoints()

	stddevRttMetric := scopeMetrics.AppendEmpty()
	stddevRttMetric.SetName("ping.rtt.stddev")
	stddevRttMetric.SetUnit("ms")
	stddevRttMetricDataPoints := stddevRttMetric.SetEmptyGauge().DataPoints()

	lossRatioMetric := scopeMetrics.AppendEmpty()
	lossRatioMetric.SetName("ping.loss.ratio")
	lossRatioMetricDataPoints := lossRatioMetric.SetEmptyGauge().DataPoints()

	for _, target := range s.targets {
		pingRes, err := s.ping(target)
		if err != nil {
			var dnsErr *net.DNSError

			if errors.As(err, &dnsErr) {
				s.logger.Log(zap.WarnLevel, "skipping target", zap.Error(dnsErr))
				continue
			} else {
				return pmetric.NewMetrics(), fmt.Errorf(
					"failed to execute pinger for target %q: %w", target.Target, err,
				)
			}
		}
		pingRes.tag = s.tag

		// Check if no packets were received (i.e., ping timed out or no response)
		/*if len(pingRes.Packets) == 0 {
			// If no packets were received, skip adding data points
			s.logger.Warn("no packets received, skipping data points for target", zap.String("target", target.Target))
			continue
		}*/

		// Add data points only if we got valid ping results
		for _, pkt := range pingRes.Packets {
			appendPacketDataPoint(rttMetricDataPoints, float64(pkt.Rtt.Nanoseconds())/1e6, pkt, pingRes)
		}

		appendStatsDataPoint(lossRatioMetricDataPoints, pingRes.Stats.PacketLoss/100., pingRes)
		appendStatsDataPoint(minRttMetricDataPoints, float64(pingRes.Stats.MinRtt)/1e6, pingRes)
		appendStatsDataPoint(maxRttMetricDataPoints, float64(pingRes.Stats.MaxRtt)/1e6, pingRes)
		appendStatsDataPoint(avgRttMetricDataPoints, float64(pingRes.Stats.AvgRtt)/1e6, pingRes)
		appendStatsDataPoint(stddevRttMetricDataPoints, float64(pingRes.Stats.StdDevRtt)/1e6, pingRes)
	}

	return metrics, nil
}

func appendPacketDataPoint(
	metricDataPoints pmetric.NumberDataPointSlice,
	value float64,
	pkt *packet,
	pingRes *pingResult,
) {
	stats := pingRes.Stats
	dp := metricDataPoints.AppendEmpty()
	dp.SetDoubleValue(value)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(pkt.Timestamp))
	dp.Attributes().PutStr(AttrPeerIp, pkt.Addr)
	dp.Attributes().PutStr(AttrPeerName, stats.Addr)
	dp.Attributes().PutStr(AttrTag, pingRes.tag)
}

func appendStatsDataPoint(
	metricDataPoints pmetric.NumberDataPointSlice,
	value float64,
	pingRes *pingResult,
) {
	dp := metricDataPoints.AppendEmpty()
	dp.SetDoubleValue(value)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(pingRes.StatsTimestamp))
	dp.Attributes().PutStr(AttrPeerIp, pingRes.Stats.IPAddr.IP.String())
	dp.Attributes().PutStr(AttrPeerName, pingRes.Stats.Addr)
	dp.Attributes().PutStr(AttrTag, pingRes.tag)
}

func (s *pingScraper) ping(target Target) (*pingResult, error) {
	pinger, err := probing.NewPinger(target.Target)
	if err != nil {
		return &pingResult{}, fmt.Errorf("failed to create pinger: %w", err)
	}

	res := &pingResult{}

	pinger.OnRecv = func(pkt *probing.Packet) {
		res.Packets = append(
			res.Packets,
			&packet{
				Timestamp: time.Now(),
				Packet:    pkt,
			},
		)
	}

	if target.PingCount != nil {
		pinger.Count = *target.PingCount
	} else {
		pinger.Count = s.defaultPingCount
	}

	if target.PingTimeout != nil {
		pinger.Timeout = *target.PingTimeout
	} else {
		pinger.Timeout = s.defaultPingTimeout
	}

	err = pinger.Run()
	if err != nil {
		return &pingResult{}, fmt.Errorf("failed to run pinger: %w", err)
	}

	res.Stats = pinger.Statistics()
	res.StatsTimestamp = time.Now()

	return res, nil
}
