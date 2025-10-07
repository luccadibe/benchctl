#!/bin/bash

# Load generator script for the local container example
# This script generates load against the HTTP server and outputs CSV data

set -e

# Configuration
SERVER_URL="${SERVER_URL:-http://localhost:8080}"
DURATION="${DURATION:-10}"  # seconds
RATE="${RATE:-10}"         # requests per second
OUTPUT_FILE="${OUTPUT_FILE:-/tmp/load_test_results.csv}"

echo "Starting load generator..."
echo "Target: $SERVER_URL"
echo "Duration: ${DURATION}s"
echo "Rate: ${RATE} req/s"
echo "Output: $OUTPUT_FILE"

# Create output directory if it doesn't exist
mkdir -p "$(dirname "$OUTPUT_FILE")"

# Initialize CSV file with headers
echo "timestamp,latency_ms,status,response_time_ms,task_type" > "$OUTPUT_FILE"

# Function to make a request and record metrics
make_request() {
    local start_time=$(date +%s.%N)
    local timestamp=$(date +%s)

    # Make the request and capture both response body and HTTP metadata
    local response_body=$(curl -s "$SERVER_URL/work" 2>/dev/null)
    local http_code=$(curl -s -w "%{http_code}" -o /dev/null "$SERVER_URL/work" 2>/dev/null || echo "000")

    local end_time=$(date +%s.%N)

    # Parse task_type from JSON response body
    local task_type="unknown"
    if [ "$http_code" = "200" ] && [ -n "$response_body" ]; then
        # Use jq if available, otherwise fallback to basic parsing
        if command -v jq >/dev/null 2>&1; then
            task_type=$(echo "$response_body" | jq -r '.task_type // "unknown"' 2>/dev/null || echo "unknown")
        else
            # Basic JSON parsing fallback
            task_type=$(echo "$response_body" | grep -o '"task_type":[[:space:]]*[0-9]*' | grep -o '[0-9]*' || echo "unknown")
        fi
    fi

    # Calculate latency in milliseconds (time_total from curl metadata)
    local time_total=$(curl -s -w "%{time_total}" -o /dev/null "$SERVER_URL/work" 2>/dev/null || echo "0.000")
    local latency_ms=$(echo "$time_total * 1000" | bc -l 2>/dev/null | cut -d'.' -f1 || echo "0")

    # Determine status
    local status="success"
    if [ "$http_code" != "200" ]; then
        status="error"
        task_type="error"
    fi

    # Calculate response time in milliseconds (actual time spent)
    local response_time_ms=$(echo "($end_time - $start_time) * 1000" | bc -l 2>/dev/null | cut -d'.' -f1 || echo "0")

    # Write to CSV
    echo "$timestamp,$latency_ms,$status,$response_time_ms,$task_type" >> "$OUTPUT_FILE"
}

# Function to wait for server to be ready
wait_for_server() {
    echo "Waiting for server to be ready..."
    local max_attempts=30
    local attempt=0
    
    while [ $attempt -lt $max_attempts ]; do
        if curl -s "$SERVER_URL/health" > /dev/null 2>&1; then
            echo "Server is ready!"
            return 0
        fi
        echo "Attempt $((attempt + 1))/$max_attempts: Server not ready, waiting..."
        sleep 2
        attempt=$((attempt + 1))
    done
    
    echo "ERROR: Server did not become ready within $((max_attempts * 2)) seconds"
    exit 1
}

# Wait for server to be ready
wait_for_server

# Calculate sleep time between requests
sleep_time=$(echo "scale=6; 1 / $RATE" | bc -l)

echo "Starting load test..."
echo "Sleep time between requests: ${sleep_time}s"

# Run the load test
start_time=$(date +%s)
end_time=$((start_time + DURATION))

request_count=0

while [ $(date +%s) -lt $end_time ]; do
    make_request
    request_count=$((request_count + 1))
    
    # Sleep between requests
    sleep "$sleep_time"
done

echo "Load test completed!"
echo "Total requests: $request_count"
echo "Results saved to: $OUTPUT_FILE"

# Show summary statistics
if [ -f "$OUTPUT_FILE" ]; then
    total_lines=$(wc -l < "$OUTPUT_FILE")
    success_count=$(tail -n +2 "$OUTPUT_FILE" | cut -d',' -f3 | grep -c "success" || echo "0")
    error_count=$(tail -n +2 "$OUTPUT_FILE" | cut -d',' -f3 | grep -c "error" || echo "0")
    
    echo "Summary:"
    echo "  Total requests: $((total_lines - 1))"
    echo "  Successful: $success_count"
    echo "  Errors: $error_count"
    
    if [ $((total_lines - 1)) -gt 0 ]; then
        success_rate=$(echo "scale=2; $success_count * 100 / ($total_lines - 1)" | bc -l)
        echo "  Success rate: ${success_rate}%"
    fi
fi
