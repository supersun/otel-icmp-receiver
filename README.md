# OpenTelemetry Collector - ICMP Check Receiver

#### Original work is from [here](github.com/thmshmm/icmpcheckreceiver)

This receiver executes ICMP echo/ping requests to a list of targets and reports the results as metrics.

---

## Overview

This is an OpenTelemetry Collector receiver component that performs ICMP ping checks against configured targets and emits metrics about network connectivity and latency. It helps monitor network reachability and performance.

### Architecture

#### Core Components

**Factory** (`factory.go`)

- Registers the receiver with the OpenTelemetry Collector
- Creates the receiver instance from configuration
- Uses `scraperhelper` for periodic collection

**Configuration** (`config.go`)

- `Target`: IP address or hostname, optional ping count and timeout overrides
- `Config`: collection interval, default ping count/timeout, targets list, optional tag
- Validation: prevents duplicate targets, validates ping parameters, ensures minimum values

**Scraper** (`receiver.go`)

- Core logic: periodically pings configured targets and collects statistics
- For each target:
    - Uses `github.com/prometheus-community/pro-bing` library to execute ICMP pings
    - Collects packet-level RTT data and aggregate statistics
    - Handles DNS errors gracefully (logs warning, continues)
    - Emits metrics with attributes (peer IP, peer name, tag)

#### How It Works

```
┌─────────────────┐
│ Collector Start │
└────────┬────────┘
         │
         ▼
┌─────────────────────────┐
│ Factory creates Receiver │
│ with scraperhelper       │
└────────┬─────────────────┘
         │
         ▼
┌─────────────────────────┐
│ Every collection_interval│
│ (e.g., 10s)            │
└────────┬─────────────────┘
         │
         ▼
┌─────────────────────────┐
│ For each Target:        │
│ 1. Resolve hostname     │
│    (if needed)          │
│ 2. Execute ICMP ping    │
│    (pro-bing library)   │
│ 3. Collect packets &    │
│    statistics           │
│ 4. Record metrics       │
│ 5. Emit with attributes │
└────────┬─────────────────┘
         │
         ▼
┌─────────────────────────┐
│ Metrics sent to pipeline│
│ (processors → exporters)│
└─────────────────────────┘
```

#### Key Features

- **Periodic pinging**: configurable interval (e.g., every 10 seconds)
- **Multiple targets**: supports pinging multiple hosts/IPs simultaneously
- **Per-target configuration**: override ping count and timeout per target
- **Error handling**: DNS errors logged as warnings, target retried on next scrape
- **Comprehensive metrics**: RTT (per packet), min/max/avg/stddev, and packet loss ratio
- **Tagging support**: optional tag for grouping/organizing targets

#### Metric Output

Produces 6 gauge metrics (all in milliseconds except loss ratio):

1. **`ping.rtt`**: Round-trip time per packet
    - Attributes: `net.peer.ip`, `net.peer.name`, `tag`
    - One data point per packet received

2. **`ping.rtt.min`**: Minimum RTT across all packets
    - Attributes: `net.peer.ip`, `net.peer.name`, `tag`
    - One data point per target

3. **`ping.rtt.max`**: Maximum RTT across all packets
    - Attributes: `net.peer.ip`, `net.peer.name`, `tag`
    - One data point per target

4. **`ping.rtt.avg`**: Average RTT across all packets
    - Attributes: `net.peer.ip`, `net.peer.name`, `tag`
    - One data point per target

5. **`ping.rtt.stddev`**: Standard deviation of RTT
    - Attributes: `net.peer.ip`, `net.peer.name`, `tag`
    - One data point per target

6. **`ping.loss.ratio`**: Packet loss ratio (0.0 to 1.0)
    - Attributes: `net.peer.ip`, `net.peer.name`, `tag`
    - One data point per target

#### Use Cases

Useful for monitoring scenarios like:

- Network connectivity monitoring
- Latency tracking between services
- ISP/network quality monitoring
- Endpoint availability checks
- Network troubleshooting and diagnostics

This is a custom OpenTelemetry component that integrates into the collector pipeline to provide ICMP-based network monitoring as observability metrics.

---

## Features

* List of created metrics:
    * `ping_rtt`: Round-trip time in milliseconds.
    * `ping_rtt_min`: Minimum round-trip time in milliseconds.
    * `ping_rtt_max`: Maximum round-trip time in milliseconds.
    * `ping_rtt_avg`: Average round-trip time in milliseconds.
    * `ping_rtt_stddev`: Standard deviation of round-trip time in milliseconds.
    * `ping_loss_ratio`: Packet loss ratio between 0 and 1.
* Name resolution errors will be logged as warnings and the target will be retried in the next scrape run.

Example Grafana visualization:
![Grafana](./docs/grafana-metric-rtt-example.png)

## Configuration

The receiver supports the following settings:

-
`collection_interval`: The interval (duration, e.g. 1m) at which the scraper will run. See [scrapehelper](https://github.com/open-telemetry/opentelemetry-collector/blob/main/receiver/scraperhelper/config.go) for all options.
- `targets`: A list of targets to ping.
- `default_ping_count`: The number of pings to send to the target.
- `default_ping_timeout`: The timeout (duration, e.g. 5s) for this target. If
  `default_ping_count` pings are not received within this time, the execution will be stopped.

target:

- `target`: The target to ping. This can be an IP address or hostname.
- `ping_count`: The number of pings to send to the target.
- `ping_timeout`: The timeout (duration, e.g. 5s) for this target. If
  `ping_count` pings are not received within this time, the execution will be stopped.

Example configuration:

```yaml
receivers:
  icmpcheck:
    collection_interval: 10s
    default_ping_count: 3
    default_ping_timeout: 5s
    targets:
      - target: www.amazon.de
        ping_count: 4
        ping_timeout: 5s
      - target: www.amazon.com
      - target: www.doesnot123exiiiiist.coom
      - target: api.amazon.com
      - target: api.amazon.de # request timeout
        ping_timeout: 2s
      - target: 8.8.8.8
  icmpcheck-5s:
    collection_interval: 5s
    default_ping_count: 3
    default_ping_timeout: 5s
    targets:
      - target: www.google.com
        ping_count: 4
        ping_timeout: 5s
      - target: www.google.com
      - target: www.googlecomexiars.coom
      - target: api.google.com
      - target: api.google.de # request timeout
        ping_timeout: 2s
      - target: 8.8.8.8

exporters:
  debug:
    verbosity: detailed

service:
  pipelines:
    metrics:
      receivers: [ icmpcheck, icmpcheck-5s ]
      exporters: [ debug ]
```

## Usage

The receiver can be used in a [custom collector build](https://opentelemetry.io/docs/collector/custom-collector/).

Example builder manifest file:

```yaml
dist:
  name: otelcol-dev
  description: Basic OTel Collector distribution for Developers
  output_path: ./otelcol-dev
  otelcol_version: 0.139.0

exporters:
  - gomod: go.opentelemetry.io/collector/exporter/debugexporter v0.139.0

processors:
  - gomod: go.opentelemetry.io/collector/processor/batchprocessor v0.139.0

receivers:
  - gomod: github.com/supersun/otel-icmp-receiver v0.139.0
```

---

## Upgrading Between Versions

When upgrading dependencies in this repository, validate the changes locally:

### 1. Run Tests

```bash
# Run all tests to ensure compatibility
go test ./... -v

# Run specific test suites
go test ./... -run TestSuccessfulPingScrape
go test ./... -run TestLoadConfig
```

### 2. Verify Build

```bash
# Ensure the code compiles without errors
go build ./...

# If using vendor directory, sync it
go mod vendor
```

### 3. Check for Breaking Changes

```bash
# Review any compilation errors or warnings
go build ./... 2>&1 | grep -i error

# Check for deprecated API usage
go vet ./...
```

### 4. Create and Push Release Tag

After validation passes, create a new tag for the release so users can reference it in their build manifests. Tags follow the format
`v0.x.x` (e.g., `v0.128.0`, `v0.139.0`):

```bash
# Set the TAG environment variable and push (replace v0.139.0 with the appropriate version)
TAG=v0.139.0 make push-tags
```

This will create an annotated tag with the message "Version v0.139.0" and push it to the remote repository.

Users can then reference this tag in their builder manifest:

```yaml
receivers:
  - gomod: github.com/supersun/otel-icmp-receiver v0.139.0
```

### Version Compatibility

| Receiver Version | OpenTelemetry Collector Version | Notes |
|----------------|----------------------------------|-------|
| v0.139.0 | v0.139.0 | Latest version with updated dependencies |
| v0.128.x | v0.128.0 | Previous stable version |

**Note
**: Always check the [OpenTelemetry Collector release notes](https://github.com/open-telemetry/opentelemetry-collector/releases) for breaking changes when upgrading between major or minor versions.

### Common Breaking Changes to Watch For

When upgrading OpenTelemetry Collector versions, watch for:

1. **Deprecated API removal**: Check for removed interfaces or methods (e.g., `pipeline.Signal` was removed in v0.139.0)
2. **Configuration format changes**: Telemetry configuration formats may change
3. **Import path changes**: Package locations may be reorganized
4. **Interface changes**: Scraper interfaces may evolve

### Update Checklist

- [ ] Update dependencies using one of these methods:
    - Option 1: Use make target: `make update-deps` (runs `go get -u -v ./...` and `go mod tidy`)
    - Option 2: Update all dependencies to latest: `go get -u -v ./...` then `go mod tidy`
    - Option 3: Manually update
      `go.mod` with specific versions (v1.45.0 for v1.x packages, v0.139.0 for v0.x packages) then `go mod tidy`
- [ ] Remove any deprecated code (e.g., `pipeline.Signal` interface)
- [ ] Update vendor directory: `go mod vendor`
- [ ] Run tests: `go test ./... -v`
- [ ] Verify build: `go build ./...`
- [ ] Check for linter errors
- [ ] Update README version references
- [ ] Create and push release tag: `TAG=v0.139.0 make push-tags`

---
