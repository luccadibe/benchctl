# Plotting Configuration

This document describes how to configure plots in `benchctl`.

## Basics

- Define plots under `plots:` in your benchmark YAML.
- `source` must match a stage output `name` (CSV).
- Supported `type`: `time_series`, `histogram`, `boxplot`.
- `engine`: `seaborn` (default) or `gonum`.
- `export_path`: relative paths are resolved under the run directory; e.g., `./plots/foo.png` -> `<run_dir>/plots/foo.png`.

## Common fields

- `title`: Plot title
- `x`, `y`: Column names from the CSV
- `groupby`: Optional column to split the plot by category (seaborn engine only)
- `format`: `png`, `svg`, or `pdf`

## Seaborn engine options (`plots[].options`)

Sizing (pixels only):
- `width_px`: integer (default 1200)
- `height_px`: integer (default 600)
- `dpi`: integer (default 150)

Styling:
- `style`: seaborn theme (e.g., `whitegrid`)
- `legend`: boolean (show/hide legend)
- `legend_loc`: location string (e.g., `best`, `upper right`)

Performance (sampling):
- Time series: `max_points` (int), `sampling`: `stride`|`random`, `random_state` (int)
- Hist/Box: `max_rows` (int), `random_state` (int)

Histogram overlays:
- `hist_element`: override seaborn's `element` when grouping (default `step`)
- `hist_common_norm`: boolean to normalize grouped histograms together (`false` by default when grouping)

Time series specifics:
- `x_label_angle`: float degrees to rotate x-axis labels (e.g., 45)
- `x_timestamp_format`: `full`|`medium`|`short`
  - `full`: `%Y-%m-%d %H:%M:%S`
  - `medium`: `%H:%M:%S`
  - `short`: `%M:%S`

Timestamp parsing hints from data schema:
- In `stages[].outputs[].data_schema`, for a timestamp column you can specify:
  - `unit`: `s|ms|us|ns`
  - `format`: `unix|unix_ms|unix_us|unix_ns|rfc3339|rfc3339_nano|iso8601`
- If omitted, benchctl auto-detects timestamp format at plot time and logs a warning; auto-detection may be slower or ambiguous.

## Examples

Time series with downsampling and angled timestamp labels:

```yaml
plots:
  - name: latency_over_time
    title: Request Latency Over Time
    source: load_test_results
    type: time_series
    x: timestamp
    y: latency_ms
    format: png
    export_path: ./plots/latency_over_time.png
    engine: seaborn
    options:
      style: whitegrid
      dpi: 150
      width_px: 1200
      height_px: 600
      max_points: 5000
      sampling: stride   # or "random"
      x_label_angle: 45
      x_timestamp_format: medium

Time series grouped by a categorical column (seaborn engine only):

```yaml
  - name: latency_by_pod
    title: Latency (P95) by Pod
    source: load_test_results
    type: time_series
    x: timestamp
    y: latency_ms
    groupby: pod_name
    format: png
    engine: seaborn
    options:
      legend: true
      legend_loc: upper right
      max_points: 2000
```

When `groupby` is set, seaborn uses it as the hue/category column—producing separate line series, histogram overlays, or grouped box plots for each distinct value.

Histogram with sampling:

```yaml
  - name: latency_distribution
    title: Latency Distribution
    source: load_test_results
    type: histogram
    x: latency_ms
    export_path: ./plots/latency_distribution.png
    engine: seaborn
    options:
      bins: 30
      max_rows: 100000
```

Boxplot:

```yaml
  - name: latency_boxplot
    title: Server Latency by Task Type
    source: load_test_results
    type: boxplot
    x: task_type
    y: latency_ms
    export_path: ./plots/latency_boxplot.png
    engine: seaborn
    options:
      width_px: 900
      height_px: 500
```
