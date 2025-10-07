# Local Container Example

This example demonstrates the benchctl framework with a local containerized HTTP server and load testing scenario.

## Overview

This example includes:
- A Go HTTP server that simulates work with configurable delays and error rates
- Docker containerization of the server
- A load generator script
- Automated benchmark execution with data collection and visualization


## Prerequisites

- Docker installed and running
- Go 1.25.1+
- `curl` and `bc` commands available
- benchctl built and available

## Server Features

The HTTP server provides:
- `/health` - Health check endpoint
- `/work` - Main work endpoint with simulated processing
- `/metrics` - Prometheus-style metrics endpoint

## Load Generator

The load generator script (`load_generator.sh`) provides:
- Configurable request rate and duration
- CSV output with timestamp, latency, status, and response time
- Automatic server health checking
- Summary statistics

## Benchmark Stages

The benchmark configuration includes four stages:

1. **build-docker-image** - Builds the Docker image for the server
2. **start-container** - Starts the containerized server with health checks
3. **run-load-test** - Executes the load generator and collects CSV data
4. **cleanup-container** - Stops and removes the container

## Data Schema

The benchmark collects the following metrics:
- `timestamp` - Unix timestamp of the request
- `latency_ms` - Server-reported processing time in milliseconds
- `status` - Request status (success/error)
- `response_time_ms` - Total response time including network latency
- `task_type` - Type of task (0, 1, 2)

## Generated Plots

The benchmark generates three visualization plots:
- **Latency Over Time** - Time series showing latency trends
- **Latency Distribution** - Histogram showing latency distribution
- **Latency Box Plot** - Statistical summary of latency data

## Running the Example

### Quick Start

From the benchctl root directory:

```bash
# Build benchctl (if not already done)
just build

# Run the full benchmark
just example-local-container

# Clean up when done
just example-local-container-clean
```