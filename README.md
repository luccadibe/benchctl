# Benchctl

A CLI framework for orchestrating benchmarks across distributed or local setups. 
Designed for research benchmarking scenarios with workflow coordination, data collection, visualization, and result management.

## Motivation

As a part of my studies and work, I had to write many different benchmarks.
And consistently, I lost a lot of time in plumbing work:
- Managing a bunch of different ssh connections to different VMs
- Copying files over different filesystems
- Running commands everywhere
- Plotting data 
- Remembering which results belong to certain parameters used for the benchmark runs
- Managing metadata

So I decided to write a framework that would take care of all of this for me.
It ended up turning into a specialized "workflow engine" of sorts.
I also looked into Apache Airflow, but it was too complex for this use case.

## Features

- **Distributed Execution**: Run benchmarks across multiple remote hosts or locally
- **YAML Configuration**: Declarative workflow definition with hosts, stages, and plots
- **Health Checks**: Built-in readiness detection (port, HTTP, file, process, command)
- **Data Collection**: Automatic file collection via SCP with schema validation
- **Visualization**: Auto-generated plots (time series, histograms, boxplots)
- **Metadata Tracking**: Custom metadata support for benchmark runs.
- **Result Management**: Organized storage with run IDs and comprehensive metadata, so you always know exactly which parameters and configuration was used for a specific benchmark run.
- **Append Metadata from Stages**: Stages can emit JSON on stdout and append it to run metadata automatically
- **Live Command Streaming**: Stage commands stream directly to your terminal with preserved ANSI colors locally and over SSH

## Installation

### Using Arch Linux

```bash
yay -S benchctl-bin
```

### Any other OS
Go to the [releases](https://github.com/luccadibe/benchctl/releases) page and download the latestbinary for your OS.

## Quick Start

1. **Create Configuration** (`benchmark.yaml`):
```yaml
benchmark:
  name: my-benchmark
  output_dir: ./results

hosts:
  local: {}  # Local execution
  server1:
    ip: 192.168.1.100
    username: user
    key_file: ~/.ssh/id_rsa

stages:
  - name: setup
    host: local
    command: echo "Setting up benchmark..."
    
  - name: start-server
    host: server1
    command: docker run -d -p 8080:8080 my-server:latest
    health_check:
      type: port
      target: "8080"
      timeout: 30s

  - name: run-load-test
    host: local
    script: load-generator.sh
    outputs:
      - name: results
        remote_path: /tmp/results.csv
        data_schema:
          format: csv
          columns:
            - name: timestamp
              type: timestamp
              unit: s
              format: unix
            - name: latency_ms
              type: float
              unit: ms

plots:
  - name: latency-over-time
    title: Request Latency Over Time
    source: results
    type: time_series
    x: timestamp
    y: latency_ms
    engine: seaborn
    format: png
    export_path: ./plots/latency_over_time.png
    options:
      dpi: 150
      width_px: 1200
      height_px: 600
      x_label_angle: 45
      x_timestamp_format: medium
```

2. **Run Benchmark**:
```bash
benchctl run --config benchmark.yaml
```

3. **View Results**:
```bash
# Results saved to ./results/1/...
# Check metadata.json for run details
# Generated plots in ./results/1/plots/
```

## Plotting engines

- Default engine: `seaborn` (Python) via `uv run` with an embedded script (PEP 723).
- Alternative: `gonum` (pure Go).

### Python/uv requirements (for seaborn)

- You need `python >= 3.10` and [uv](https://docs.astral.sh/uv/) available on your PATH.
- First run downloads Python deps into uv’s cache; subsequent runs are fast.
- No virtualenvs or repo Python files required; everything is embedded and invoked via `uv run`.

## Configuration Reference

### Hosts
Define execution environments:

```yaml
hosts:
  local: {}  # Local host
  remote:
    ip: 10.0.0.1
    username: benchmark
    key_file: ~/.ssh/benchmark_key
    password: optional_password
```

### Stages
Sequential workflow steps:

```yaml
stages:
  - name: build
    host: local
    command: make build
    
  - name: deploy
    host: remote
    script: deploy.sh
    health_check:
      type: http
      target: "http://localhost:8080/health"
    
  - name: load-test
    host: local
    script: load-test.sh
    outputs:
      - name: metrics
        remote_path: /tmp/metrics.csv
```

### Data Schema
Define CSV column types for validation and plotting:

```yaml
data_schema:
  format: csv
  columns:
    - name: timestamp
      type: timestamp
      unit: s
      format: unix   # optional; supported: unix, unix_ms, unix_us, unix_ns, rfc3339, rfc3339_nano, iso8601
    - name: latency_ms
      type: float
      unit: ms
    - name: status
      type: string
```

### Plots
Auto-generated visualizations:

```yaml
plots:
  - name: latency-histogram
    title: Latency Distribution
    source: metrics
    type: histogram
    x: latency_ms
    
  - name: throughput-timeseries
    title: Requests Over Time
    source: metrics
    type: time_series
    x: timestamp
    y: requests_per_second
```

## Usage

### Basic Commands

```bash
# Run benchmark
benchctl run --config benchmark.yaml

# Add custom metadata
benchctl run --config benchmark.yaml --metadata "someFeature"="true" --metadata "someOtherFeature"="false"

# Inspect a run
benchctl inspect <run-id>

# Add some metadata to a run
benchctl edit <run-id>  --metadata "hello"="world"

# View help
benchctl --help

# Compare two runs
benchctl compare <run-id1> <run-id2>
```

### Metadata

Append JSON metadata directly from a stage by enabling `append_metadata`.

In your configuration, you can add a stage that outputs a  some metadata in JSON format to stdout:

```yaml
stages:
  - name: analyse
    host: local
    command: |
      uv run python - <<'PY'
      # /// script
      # requires-python = ">=3.10"
      # dependencies = []
      # ///
      import json
      # Compute something and print a single JSON object
      print(json.dumps({
          "latency_p50_ms": "123.4",
          "notes": "baseline run"
      }))
      PY
    append_metadata: true
```

At runtime, the JSON keys/values are stringified and merged into the run’s metadata under `custom`.
This is helpful to annotate runs with additional metadata that is dependent on the stage's output.
It can be used to enable easy comparison of runs, for example to compare performance statistics.
You can run your data-analysis script and append the metadata to the run.

## Stage Environment Variables

During stage execution, the following environment variables are exported for commands/scripts:

- `BENCHCTL_RUN_ID`: the current run ID
- `BENCHCTL_RUN_DIR`: absolute path to the run directory (e.g., ./results/1)
- `BENCHCTL_OUTPUT_DIR`: benchmark output root (from `benchmark.output_dir`)
- `BENCHCTL_CONFIG_PATH`: set if provided in the environment when invoking benchctl
- `BENCHCTL_BIN`: absolute path to the running benchctl binary

Use these to locate inputs/outputs or to parameterize your scripts.

## Examples

See the [`examples/`](examples/) directory for complete benchmark configurations.
- Local container testing

## Roadmap

- [ ] Add a local Web UI for viewing benchmark results and plots.
- [ ] Maybe add support for data analysis? Might be too much.


## License

MIT
