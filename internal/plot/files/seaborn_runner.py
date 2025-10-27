# /// script
# requires-python = ">=3.10"
# dependencies = [
#   "pandas>=2.2.2,<3",
#   "matplotlib>=3.8,<3.9",
#   "seaborn>=0.13,<0.14",
# ]
# ///

import argparse, json, random
import pandas as pd
import seaborn as sns
import matplotlib.pyplot as plt
import matplotlib.dates as mdates

parser = argparse.ArgumentParser()
parser.add_argument("--input", required=True)
parser.add_argument("--output", required=True)
parser.add_argument("--spec", required=True)
args = parser.parse_args()

with open(args.spec) as f:
    spec = json.load(f)

df = pd.read_csv(args.input)
opts = spec.get("opts", {})
sns.set_theme(style=opts.get("style", "whitegrid"))
groupby = spec.get("groupby")

# Figure sizing: pixel-only (no inches fallback)
dpi = int(opts.get("dpi", 150))
width_px = float(opts.get("width_px", 1200))
height_px = float(opts.get("height_px", 600))
figsize = (width_px / dpi, height_px / dpi)
plt.figure(figsize=figsize, dpi=dpi)
t = spec["type"]
title = spec.get("title") or ""
fmt = spec.get("format","png")

if t == "time_series":
    x, y = spec["x"], spec["y"]
    # Parse time with explicit hints first, then fallback to heuristic autodetect
    parsed = False
    try:
        fmt_hint = opts.get("x_time_format")  # unix, unix_ms, unix_us, unix_ns, rfc3339, rfc3339_nano, iso8601
        unit_hint = opts.get("x_time_unit")   # s, ms, us, ns
        series = df[x]
        if fmt_hint:
            key = str(fmt_hint).lower()
            if key.startswith("unix"):
                unit = unit_hint or ("ns" if key == "unix_ns" else "us" if key == "unix_us" else "ms" if key == "unix_ms" else "s")
                df[x] = pd.to_datetime(pd.to_numeric(series, errors="coerce"), unit=unit, errors="coerce")
                parsed = True
            elif key in ("rfc3339", "rfc3339_nano", "iso8601"):
                df[x] = pd.to_datetime(series, errors="coerce", utc=False, infer_datetime_format=True)
                parsed = True
    except Exception:
        parsed = False

    if not parsed:
        try:
            import pandas.api.types as ptypes
            if ptypes.is_numeric_dtype(series):
                s = pd.to_numeric(series, errors="coerce")
                vmax = s.max()
                unit = None
                if vmax >= 1e18:
                    unit = "ns"
                elif vmax >= 1e15:
                    unit = "us"
                elif vmax >= 1e12:
                    unit = "ms"
                elif vmax >= 1e9:
                    unit = "s"
                if unit:
                    df[x] = pd.to_datetime(s, unit=unit, errors="coerce")
                else:
                    df[x] = pd.to_datetime(series, errors="coerce")
            else:
                df[x] = pd.to_datetime(series, errors="coerce")
        except Exception:
            pass
    # Downsampling for speed/clarity
    max_points = int(opts.get("max_points", 0))
    if max_points and len(df) > max_points:
        strategy = opts.get("sampling", "stride")  # stride|random
        if strategy == "random":
            df = df.sample(n=max_points, replace=False, random_state=opts.get("random_state"))
            df = df.sort_values(by=x)
        else:
            step = max(1, len(df) // max_points)
            df = df.iloc[::step]
    lineplot_kwargs = dict(data=df, x=x, y=y)
    if groupby:
        lineplot_kwargs["hue"] = groupby
    ax = sns.lineplot(**lineplot_kwargs)
    # If datetime, ensure date ticks/formatting are applied
    try:
        if pd.api.types.is_datetime64_any_dtype(df[x]):
            ax.xaxis.set_major_locator(mdates.AutoDateLocator())
            fmt_key = opts.get("x_timestamp_format")
            if fmt_key:
                fmts = {
                    "full": "%Y-%m-%d %H:%M:%S",
                    "medium": "%H:%M:%S",
                    "short": "%M:%S",
                }
                fmt_str = fmts.get(str(fmt_key).lower())
                if fmt_str:
                    ax.xaxis.set_major_formatter(mdates.DateFormatter(fmt_str))
    except Exception:
        pass
    # Tick angle and timestamp formatting
    angle = float(opts.get("x_label_angle", 0))
    if angle:
        ax.tick_params(axis='x', labelrotation=angle)
    # Ensure layout updates reflect rotated labels
    try:
        plt.tight_layout()
    except Exception:
        pass
elif t == "histogram":
    x = spec["x"]
    # For very large datasets, optionally sample rows
    max_rows = int(opts.get("max_rows", 0))
    if max_rows and len(df) > max_rows:
        df = df.sample(n=max_rows, replace=False, random_state=opts.get("random_state"))
    bins = int(opts.get("bins", 16))
    histplot_kwargs = dict(data=df, x=x, stat="probability", bins=bins)
    if groupby:
        histplot_kwargs["hue"] = groupby
        histplot_kwargs["element"] = opts.get("hist_element", "step")
        histplot_kwargs["common_norm"] = bool(opts.get("hist_common_norm", False))
    ax = sns.histplot(**histplot_kwargs)
elif t == "boxplot":
    x, y = spec["x"], spec["y"]
    # For very large datasets, optionally sample rows
    max_rows = int(opts.get("max_rows", 0))
    if max_rows and len(df) > max_rows:
        df = df.sample(n=max_rows, replace=False, random_state=opts.get("random_state"))
    boxplot_kwargs = dict(data=df, x=x, y=y)
    if groupby:
        boxplot_kwargs["hue"] = groupby
    ax = sns.boxplot(**boxplot_kwargs)
else:
    raise SystemExit(f"unsupported plot type: {t}")

# Legend control
legend = opts.get("legend", None)
if legend is not None:
    lg = ax.get_legend()
    if legend and lg is None:
        ax.legend(loc=opts.get("legend_loc", "best"))
    elif not legend and lg is not None:
        lg.remove()

plt.title(title)
plt.tight_layout()
plt.savefig(args.output, format=fmt)
