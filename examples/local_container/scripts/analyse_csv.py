#!/usr/bin/env python3
# /// script
# requires-python = ">=3.10"
# dependencies = [
#   "pandas>=2.2.2,<3",
# ]
# ///

import os
import sys
import json
import shutil
from collections import Counter

try:
    import pandas as pd
except Exception as e:
    print("pandas is required: " + str(e))
    sys.exit(1)


def compute_five_number_summary(series):
    s = pd.to_numeric(series, errors="coerce").dropna()
    if s.empty:
        return {}
    desc = s.describe(percentiles=[0.25, 0.5, 0.75])
    return {
        "min": float(desc["min"]),
        "p25": float(desc["25%"]),
        "median": float(desc["50%"]),
        "p75": float(desc["75%"]),
        "max": float(desc["max"]),
        "count": int(desc["count"]),
    }


def main():
    run_id = os.environ.get("BENCHCTL_RUN_ID")
    run_dir = os.environ.get("BENCHCTL_RUN_DIR")
    output_dir = os.environ.get("BENCHCTL_OUTPUT_DIR")
    if not run_id or not run_dir or not output_dir:
        print("BENCHCTL_* env vars missing; expected BENCHCTL_RUN_ID, BENCHCTL_RUN_DIR, BENCHCTL_OUTPUT_DIR")
        sys.exit(2)

    # look for the cSV in <run_id>/load_test_results.csv
    csv_path = os.path.join(run_dir, "load_test_results.csv")
    if not os.path.exists(csv_path):
        print(f"load_test_results.csv not found at {csv_path}")
        sys.exit(3)

    df = pd.read_csv(csv_path)

    five_num = {}
    if "latency_ms" in df.columns:
        five_num = compute_five_number_summary(df["latency_ms"])

    most_task = None
    if "task_type" in df.columns:
        counts = Counter(df["task_type"].astype(str))
        if counts:
            most_task, _ = counts.most_common(1)[0]

    # Prepare metadata as key=value pairs for benchctl edit
    md_pairs = []
    if five_num:
        md_pairs.append(("latency_min_ms", str(five_num.get("min"))))
        md_pairs.append(("latency_p25_ms", str(five_num.get("p25"))))
        md_pairs.append(("latency_median_ms", str(five_num.get("median"))))
        md_pairs.append(("latency_p75_ms", str(five_num.get("p75"))))
        md_pairs.append(("latency_max_ms", str(five_num.get("max"))))
        md_pairs.append(("latency_count", str(five_num.get("count"))))
    if most_task is not None:
        md_pairs.append(("most_common_task_type", str(most_task)))

    if not md_pairs:
        print(json.dumps({}))
        return

    # Print JSON to stdout for append_metadata
    print(json.dumps({k: v for k, v in md_pairs}))


if __name__ == "__main__":
    main()


