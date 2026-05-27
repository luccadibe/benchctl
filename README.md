# Benchctl

A CLI framework for orchestrating benchmarks across distributed or local setups.
Designed for research benchmarking scenarios with workflow coordination, data collection, and result management.

## Motivation

As a part of my studies and work, I had to write many different benchmarks.
And consistently, I lost a lot of time in plumbing work:
- Managing a bunch of different ssh connections to different VMs
- Copying files over different filesystems
- Running commands everywhere
- Keeping benchmark data organized
- Remembering which results belong to certain parameters used for the benchmark runs
- Managing metadata

So I decided to write a framework that would take care of all of this for me.
It ended up turning into a specialized "workflow engine" of sorts.
I also looked into Apache Airflow, but it was too complex for this use case.

## Features

- **Distributed Execution**: Run benchmarks across multiple remote hosts or locally
- **YAML Configuration**: Declarative workflow definition with hosts and stages
- **Health Checks**: Built-in readiness detection (port, HTTP, file, process, command)
- **Data Collection**: Automatic file collection via SCP
- **Background Stages**: Keep monitoring commands running alongside your benchmark until all your non-background stages finish
- **Metadata Tracking**: Custom metadata support for benchmark runs.
- **Result Management**: Organized storage with run IDs and comprehensive metadata, so you always know exactly which parameters and configuration was used for a specific benchmark run.
- **Post-Run Annotation**: Add custom metadata to completed runs after inspecting results
- **Structured Logging**: Human-readable console logs plus JSON logs in each run directory
- **Git Capture**: Automatic commit, branch, remote, and dirty-state metadata
- **Comparison Cases**: Run the same workflow over named cases with per-case environment variables
- **Result Sync**: Optional `rclone` push for backing up result directories
- **Live Command Streaming**: Stage commands stream directly to your terminal with preserved ANSI colors locally and over SSH

> **Note**: `benchctl` is under active development. There is currently **no commitment to API stability**. Features, flags, and file formats may change in future releases until I release v1.0.0.


## Installation

### Quick Install (Linux/macOS)

```bash
curl -sSL https://raw.githubusercontent.com/luccadibe/benchctl/main/install-benchctl.sh | bash
```

Or download and run manually:
```bash
wget https://raw.githubusercontent.com/luccadibe/benchctl/main/install-benchctl.sh
chmod +x install-benchctl.sh
./install-benchctl.sh
```

### Using Arch Linux

```bash
yay -S benchctl-bin
```

### Manual Installation
Go to the [releases](https://github.com/luccadibe/benchctl/releases) page and download the latest binary for your OS.

## Quick Start

1. **Create Configuration** (`benchmark.yaml`):
```yaml
benchmark:
  name: my-benchmark
  output_dir: ./results
  logging:
    level: info
  git:
    require_clean: false

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
```

### Logging

benchctl writes colored human-readable logs to the terminal and JSON logs to `benchctl.ndjson` inside each run directory by default.
Set `benchmark.logging.path` to choose a different JSON log path, `benchmark.logging.level` to `debug`, `info`, `warn`, or `error`, and optionally `benchmark.logging.time_format` for the console timestamp (Go time layout; default `15:04:05`).

### Git Metadata

Git metadata is captured automatically when `benchctl run` starts inside a git repository.

```yaml
benchmark:
  git:
    capture: true
    require_clean: false
    save_patch: false
```

Set `require_clean: true` to fail runs with a dirty worktree. Set `save_patch: true` to write `git.patch` into the run directory when tracked files are dirty.

2. **Run Benchmark**:
```bash
benchctl run --config benchmark.yaml
```

3. **View Results**:
```bash
# Results saved to ./results/1/...
# Check metadata.json for run details
# Collected files are stored directly in ./results/1/
```

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
Stages are sequential workflow steps, they are executed in the order they are defined and they must have a unique name.

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

  - name: monitor-resources
    host: local
    command: ./scripts/monitor.sh
    background: true  # keeps running until the workflow shuts it down safely
    
  - name: load-test
    host: local
    script: load-test.sh
    outputs:
      - name: metrics
        remote_path: /tmp/metrics.csv
```

#### Shell execution
Stages run through a shell command. Set `benchmark.shell` to control it (the default is `bash -lic`), which loads login + interactive environment (PATH, JAVA_HOME, etc). Override per stage with `stages[].shell`.
> **Note:** You cannot pass arguments to a script like `script.sh <args>`. Use `command` instead.

#### Hosts and multi-host stages
- Use `host` for a single host or `hosts` for multiple hosts. If neither is set, the stage runs on `local`.
- Hosts in `hosts` execute sequentially in the listed order.
- For multi-host output collection, include `${BENCHCTL_HOST}` in `outputs[].name` (and matching `remote_path` on each host) so files do not overwrite each other.

Example:
```yaml
stages:
  - name: run-everywhere
    hosts: [vm1, vm2]
    command: uname -a > /tmp/${BENCHCTL_HOST}-uname.txt
    outputs:
      - name: ${BENCHCTL_HOST}-uname
        remote_path: /tmp/${BENCHCTL_HOST}-uname.txt
```

#### Skipping stages
- Set `stages[].skip: true` to skip a stage.
- Or pass `benchctl run --skip <stage-name>` multiple times (CLI overrides config).
- The `metadata.json` that is stored in each run directory will contain the exact stages that were executed, so you can easily see which stages were executed and which were skipped.

Background stages run alongside the rest of the workflow. benchctl keeps them alive until the final non-background stage finishes, then sends SIGTERM to the stage's process group, waits `BackgroundTerminationGrace` (2 seconds by default), and finally SIGKILL if they are still running.
This uses `setsid` to start a new process group, so the entire background task tree is terminated reliably.
Their outputs are collected after shutdown, so its ideal for monitoring tasks, like resource usage monitoring.

### Comparison Cases

Use `cases:` to run the same stages for multiple named benchmark variants. Each case exports `BENCHCTL_CASE_NAME` plus its configured `env` values.

```yaml
cases:
  - name: postgres
    env:
      DB_ENGINE: postgres
  - name: mysql
    env:
      DB_ENGINE: mysql

stages:
  - name: run-load-test
    command: ./load.sh "$DB_ENGINE"

  - name: postgres-extra
    execute_only_for: postgres
    command: ./postgres-extra.sh
```

Use `${DB_ENGINE}` (or other case `env` keys) in `outputs[].name` and `outputs[].remote_path` so each case writes and collects distinct files, for example `postgres-metrics.csv` and `mysql-metrics.csv`.

### Sync

benchctl delegates result sync to [`rclone`](https://rclone.org/). Configure the destination in `benchmark.yaml`:

```yaml
benchmark:
  sync:
    remote: s3:my-bucket/benchctl-results
    args: ["--checksum"]
```

Then run:

```bash
benchctl sync push --config benchmark.yaml
```

## Usage

### Basic Commands

```bash
# Run benchmark
benchctl run --config benchmark.yaml

# Skip stages by name
benchctl run --config benchmark.yaml --skip setup --skip warmup

# Add custom metadata when starting a run
benchctl run --config benchmark.yaml --metadata "someFeature"="true" --metadata "someOtherFeature"="false"

# Pass environment variables to stages
benchctl run --config benchmark.yaml -e BRANCH=main -e LG_MAX_RPS=2000

# Inspect a run
benchctl inspect <run-id>

# Annotate a completed run after analysis
benchctl annotate <run-id> --metadata latency_p95_ms=123.4
```


### Metadata

Pass `--metadata key=value` to `benchctl run` for metadata known before execution.
Use `benchctl annotate <run-id> --metadata key=value` after a run for metadata discovered during ad hoc analysis.

## Stage Environment Variables

During stage execution, the following environment variables are exported for commands/scripts:

- `BENCHCTL_RUN_ID`: the current run ID
- `BENCHCTL_RUN_DIR`: absolute path to the run directory (e.g., ./results/1)
- `BENCHCTL_OUTPUT_DIR`: benchmark output root (from `benchmark.output_dir`)
- `BENCHCTL_CONFIG_PATH`: set if provided in the environment when invoking benchctl
- `BENCHCTL_BIN`: absolute path to the running benchctl binary
- `BENCHCTL_CASE_NAME`: current case name when `cases:` are configured
- `BENCHCTL_HOST`: host alias for the current stage execution (`stages[].host` or entry in `stages[].hosts`)

Use these to locate inputs/outputs or to parameterize your scripts.

### Output path templates

`stages[].outputs[].name` and `stages[].outputs[].remote_path` support `$VAR` and `${VAR}` expansion using the same variables as stage commands (including case `env` and CLI `-e` overrides). Collected files are stored in the run directory as `<expanded-name><extension-from-remote_path>`.

```yaml
cases:
  - name: openfaas
    env:
      BENCH_PLATFORM: openfaas

stages:
  - name: run
    host: eval-vm
    script: ./run.sh
    outputs:
      - name: ${BENCH_PLATFORM}-sustained
        remote_path: /tmp/results/${BENCH_PLATFORM}-sustained.csv
```

Undefined variables fail the run at collection time. Use `$$` for a literal `$`.

## Examples

See the [`examples/`](examples/) directory for complete benchmark configurations.
- Local container testing

## License

MIT
