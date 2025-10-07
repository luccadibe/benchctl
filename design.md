# Benchmark Coordination Framework Design Document

## Overview
A Go-based CLI framework for orchestrating distributed benchmarks across pre-existing VMs or completely local setups. Provides workflow coordination, data collection, visualization, and result management specifically tailored for research benchmarking scenarios.

## Core Components

### 1. CLI Tool (`benchctl`)
**Primary Operations:**
- `benchctl run <config.yaml>` - Execute benchmark workflow
- `benchctl plot <run-id>` - Generate plots from collected data
- `benchctl list` - Show available benchmark runs and metadata

### 2. Web UI Server (`benchctl serve`)
Separate binary providing local HTTP server with:
- Visualization of collected data
- Comparison between benchmark runs
- Metadata browsing interface

## Configuration Structure

### YAML Configuration Format
```yaml
benchmark:
  name: string
  output_dir: string              # Where results/plots are stored . by default it is under results subdirectory of current directory. will be created if it doesn't exist.
  
hosts:
  <alias>:
    ip: string # required
    username: string # required
    password: string # optional
    key_file: string # required
    key_password: string # optional
  local:           # Local host reference, doesnt need any configuration

# data_schema is now moved to stages[].outputs[].data_schema

stages:
  - name: string # name of the stage. This needs to be unique within the benchmark.
    host: string                  # Host alias from hosts section. If ommited, local host is used.
    # At least one of command or script needs to be provided. Cannot be both.
    command: string               # Shell command to execute. This will be executed in the host.
    script: string               # Script to execute. This will be executed in the host.
    health_check:                 # Readiness detection (optional)
      type: [port|http|file|process|command]
      target: string
      timeout: duration
      retries: integer
    outputs:                      # Files to collect (optional)
      - name: string              # Unique name for this output (required)
        remote_path: string
        local_path: string # local path to save the file. if not provided, the file will be saved in the output_dir for the benchmark run. like ./results/run-001/some_file.txt
        data_schema:              # Schema definition for the output file (optional)
          format: csv # just csv for now
          <header_name>:          # Column definitions for CSV output
            - type: [timestamp|integer|float|string]
            - unit: string (optional) # for example ms, ns for time , or custom string for other units like bytes, etc. This needs to be clearly defined in the documentation. Like for example bytes, B, KB, MB, GB, etc. This can then be used for plotting and data analysis.

plots: # optional
  - name: string # name of the plot. This needs to be unique within the benchmark.
    title: string # title of the plot. This will be used as the title of the plot.
    source: string # name of the output to use as data source
    type: [time_series|histogram|boxplot]
    x: [string] # x axis column name. This needs to be a valid column name in the data_schema.
    y: [string] # y axis column name. This needs to be a valid column name in the data_schema.
    aggregation: [avg|median|p95|p99]
    format: [png|svg|pdf]
    export_path: string # path to save the plot. if not provided, the plot will be saved in the output_dir for the benchmark run. like ./results/run-001/plots/some_plot.png
```

### Directory Convention
```
benchmark-project/
├── config.yaml
├── scripts/
│   ├── setup-db.sh
│   ├── start-app.sh
│   └── load-gen.sh
└── results/
    ├── run-001/
    │   ├── metadata.json
    │   ├── data.csv
    │   └── plots/
    └── run-002/
        └── ...
```

## Workflow Execution Model

### Stages
- Stages are executed in the order they are defined in the configuration file.
- Initially no parallel execution is supported.

### Health Check Types
- **port**: TCP port listening check
- **http**: HTTP endpoint availability (200 response)
- **file**: File existence check
- **command**: Custom shell command (exit code 0 = ready)

### Data Collection
- SCP files from remote hosts after workflow completion
- Files organized by run ID with metadata

## Built-in Utilities

### File Operations
- Copy files between arbitrary hosts
- Template variable substitution in commands (e.g., `{{hosts.db}}`)

### Plots
**Auto-generated plots based on CSV schema:**
- **Time Series**: Latency/throughput over time with configurable aggregation
- **Distribution**: Boxplots, histograms for numeric columns
- Configurable aggregation windows (avg, median, p95, p99)
- Built-in color palettes for multi-run comparisons
- Export formats: PNG, SVG, PDF

### Results
- Unique run IDs with timestamps
- Metadata storage inside the correct directory (benchmark config, start/end times, host info)
- Organized file structure

## Implementation

### Execution Flow
1. Parse and validate YAML configuration
2. Execute stages in sequence order
3. Run health checks to determine completion
4. Collect output files via SCP
5. Generate metadata and organize results
6. Optionally generate plots immediately

### Error Handling
- Stage failures abort workflow execution
- Detailed error logging with stage context
- Failed runs stored with error metadata for debugging and GOOD error messages in the cli output to debug quickly.