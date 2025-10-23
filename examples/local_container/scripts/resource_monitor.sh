#!/bin/bash

if [ -z "$1" ]; then
    echo "Usage: $0 output.csv"
    exit 1
fi

CSV_FILE="$1"

# Rewrite output file with header on each run
echo "timestamp_ms,cpu_used_raw,cpu_total_raw,cpu_percent,mem_used_bytes,mem_total_bytes,mem_percent" > "$CSV_FILE"

get_cpu_usage() {
    read cpu user nice system idle iowait irq softirq steal guest < /proc/stat
    total=$((user + nice + system + idle + iowait + irq + softirq + steal))
    idle=$idle
    echo "$total $idle"
}

get_mem_usage() {
    mem_total_kb=$(grep MemTotal /proc/meminfo | awk '{print $2}')
    mem_available_kb=$(grep MemAvailable /proc/meminfo | awk '{print $2}')
    mem_used_kb=$((mem_total_kb - mem_available_kb))
    mem_percent=$((100 * mem_used_kb / mem_total_kb))
    mem_used_bytes=$((mem_used_kb * 1024))
    mem_total_bytes=$((mem_total_kb * 1024))
    echo "$mem_used_bytes $mem_total_bytes $mem_percent"
}

while true; do
    read total1 idle1 <<< "$(get_cpu_usage)"
    sleep 0.5
    read total2 idle2 <<< "$(get_cpu_usage)"

    total_diff=$((total2 - total1))
    idle_diff=$((idle2 - idle1))
    cpu_used=$((total_diff - idle_diff))
    cpu_percent=$((100 * cpu_used / total_diff))

    read mem_used mem_total mem_percent <<< "$(get_mem_usage)"

    timestamp=$(date +%s%3N)

    echo "$timestamp,$cpu_used,$total_diff,$cpu_percent,$mem_used,$mem_total,$mem_percent" >> "$CSV_FILE"
done
