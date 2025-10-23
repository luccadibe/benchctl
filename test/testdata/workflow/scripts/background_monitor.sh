#!/bin/sh

set -eu

OUTPUT_FILE="/tmp/benchctl-bg.csv"

echo "timestamp,counter" > "$OUTPUT_FILE"

trap 'exit 0' TERM INT

count=0
while true; do
  count=$((count + 1))
  printf "%s,%s\n" "$(date +%s)" "$count" >> "$OUTPUT_FILE"
  sleep 0.2
done
