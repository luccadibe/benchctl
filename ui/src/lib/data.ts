import type { DataColumn, DataSchema, Output, RunMetadata } from "../types";

export function outputFileName(output: Output) {
  const extMatch = output.remote_path?.match(/(\.[^/.]+)$/);
  const ext = extMatch ? extMatch[1] : "";
  return `${output.name}${ext}`;
}

export function buildSchemaMap(meta?: RunMetadata) {
  const map = new Map<string, DataSchema>();
  const outputs: Output[] = [];
  meta?.config?.stages?.forEach((stage) => {
    stage.outputs?.forEach((output) => outputs.push(output));
  });
  outputs.forEach((output) => {
    const filename = outputFileName(output);
    if (output.data_schema) {
      map.set(filename, output.data_schema);
    }
  });
  return map;
}

export function detectFileKind(name: string) {
  const ext = name.split(".").pop()?.toLowerCase();
  if (ext === "csv") return "csv";
  if (ext === "png" || ext === "jpg" || ext === "jpeg") return "image";
  return "other";
}

export function getColumnsFromSchema(schema?: DataSchema) {
  return schema?.columns ?? [];
}

export function isNumericColumn(col: DataColumn) {
  return col.type === "integer" || col.type === "float";
}

export function isTimestampColumn(col: DataColumn) {
  return col.type === "timestamp";
}

export function parseTimestamp(value: string | undefined | null, format?: string, unit?: string) {
  const trimmed = (value ?? "").trim();
  if (!trimmed) {
    return null;
  }
  const numeric = Number(trimmed);
  const unitScale: Record<string, number> = {
    s: 1000,
    ms: 1,
    us: 1 / 1000,
    ns: 1 / 1_000_000,
  };
  if (format && format.startsWith("unix") && Number.isFinite(numeric)) {
    switch (format) {
      case "unix":
        return numeric * 1000;
      case "unix_ms":
        return numeric;
      case "unix_us":
        return numeric / 1000;
      case "unix_ns":
        return numeric / 1_000_000;
      default:
        break;
    }
  }
  if (unit && Number.isFinite(numeric) && unitScale[unit]) {
    return numeric * unitScale[unit];
  }
  const parsed = Date.parse(trimmed);
  if (!Number.isNaN(parsed)) {
    return parsed;
  }
  if (Number.isFinite(numeric)) {
    return numeric;
  }
  return null;
}

export function parseNumeric(value: string | undefined | null) {
  const num = Number(value);
  if (!Number.isFinite(num)) {
    return null;
  }
  return num;
}

function percentile(sorted: number[], p: number) {
  if (sorted.length === 0) return null;
  const idx = Math.ceil((p / 100) * sorted.length) - 1;
  return sorted[Math.min(Math.max(idx, 0), sorted.length - 1)];
}

export function calcStats(values: number[]) {
  const sorted = [...values].sort((a, b) => a - b);
  const count = sorted.length;
  if (count === 0) {
    return null;
  }
  const sum = sorted.reduce((acc, v) => acc + v, 0);
  return {
    count,
    min: sorted[0],
    mean: sum / count,
    median: percentile(sorted, 50),
    p95: percentile(sorted, 95),
    p99: percentile(sorted, 99),
    max: sorted[count - 1],
  };
}

export function applyRollingMean(values: (number | null)[], windowSize: number) {
  if (windowSize <= 1) {
    return values;
  }
  const out: (number | null)[] = [];
  let sum = 0;
  let count = 0;
  const queue: (number | null)[] = [];
  for (const val of values) {
    queue.push(val);
    if (val !== null) {
      sum += val;
      count += 1;
    }
    if (queue.length > windowSize) {
      const removed = queue.shift();
      if (removed !== undefined && removed !== null) {
        sum -= removed;
        count -= 1;
      }
    }
    out.push(count === 0 ? null : sum / count);
  }
  return out;
}
