import { useEffect, useMemo, useState } from "react";
import Panel from "../components/Panel";
import Tag from "../components/Tag";
import Table from "../components/Table";
import Chart from "../components/Chart";
import EmptyState from "../components/EmptyState";
import layout from "../styles/layout.module.css";
import { buildSchemaMap, calcStats, isNumericColumn, parseNumeric } from "../lib/data";
import { formatDateTime, formatNumber } from "../lib/format";
import { fetchCsv, fetchRunDetail } from "../lib/store";
import { getCssVar } from "../lib/dom";
import type { RunSummary } from "../types";
import Link from "../router/Link";

type Metric = "mean" | "median" | "p95" | "p99";

type DashboardViewProps = {
  runs: RunSummary[] | null;
  loading: boolean;
  error: string | null;
  theme: "light" | "dark";
};

const palette = ["#1f9e8f", "#f0a23b", "#2b7db6", "#e06c75", "#3c9d78", "#c19a6b"];

export default function DashboardView({ runs, loading, error, theme }: DashboardViewProps) {
  const [file, setFile] = useState("");
  const [column, setColumn] = useState("");
  const [metric, setMetric] = useState<Metric>("p95");
  const [limit, setLimit] = useState(8);
  const [rows, setRows] = useState<RunSummary[]>([]);
  const [statsMap, setStatsMap] = useState<Map<string, ReturnType<typeof calcStats>>>(
    new Map(),
  );
  const [chartData, setChartData] = useState<{ x: number[]; y: number[] } | null>(null);
  const [statsError, setStatsError] = useState<string | null>(null);

  const latestRun = runs?.[0];
  const [latestDetail, setLatestDetail] = useState<Awaited<ReturnType<typeof fetchRunDetail>> | null>(
    null,
  );

  useEffect(() => {
    if (!latestRun) {
      setLatestDetail(null);
      return;
    }
    let active = true;
    fetchRunDetail(latestRun.id)
      .then((detail) => {
        if (active) {
          setLatestDetail(detail);
        }
      })
      .catch((err: Error) => {
        if (active) {
          setStatsError(err.message);
        }
      });
    return () => {
      active = false;
    };
  }, [latestRun]);

  const csvFiles = useMemo(() => {
    return latestDetail?.files.filter((f) => f.kind === "csv").map((f) => f.name) ?? [];
  }, [latestDetail]);

  const schemaMap = useMemo(() => buildSchemaMap(latestDetail?.metadata), [latestDetail]);
  const schema = schemaMap.get(file);
  const columnOptions = useMemo(() => {
    return (schema?.columns ?? []).filter(isNumericColumn).map((col) => col.name);
  }, [schema]);

  useEffect(() => {
    if (!csvFiles.length) {
      return;
    }
    if (!file || !csvFiles.includes(file)) {
      setFile(csvFiles[0]);
    }
  }, [csvFiles, file]);

  useEffect(() => {
    if (!columnOptions.length) {
      return;
    }
    if (!column || !columnOptions.includes(column)) {
      setColumn(columnOptions[0]);
    }
  }, [columnOptions, column]);

  useEffect(() => {
    if (!runs || !runs.length || !file || !column) {
      setRows([]);
      setChartData(null);
      setStatsMap(new Map());
      return;
    }
    let active = true;
    setStatsError(null);
    const load = async () => {
      const selectedRuns = runs.slice(0, limit);
      const nextRows: RunSummary[] = [];
      const nextStats = new Map<string, ReturnType<typeof calcStats>>();
      const chartX: number[] = [];
      const chartY: number[] = [];

      for (const run of selectedRuns) {
        const detail = await fetchRunDetail(run.id);
        const fileInfo = detail.files.find((f) => f.name === file);
        if (!fileInfo) {
          continue;
        }
        const csv = await fetchCsv(run.id, file);
        const values = csv.rows
          .map((row) => parseNumeric(row[column]))
          .filter((val): val is number => val !== null);
        const stats = calcStats(values);
        if (!stats) {
          continue;
        }
        nextRows.push(run);
        nextStats.set(run.id, stats);

        const metricValue = stats[metric];
        if (metricValue !== null && metricValue !== undefined) {
          const startMs = Date.parse(run.start_time);
          if (!Number.isNaN(startMs)) {
            chartX.push(startMs);
            chartY.push(metricValue);
          }
        }
      }

      if (active) {
        setRows(nextRows);
        setStatsMap(nextStats);
        setChartData(chartX.length ? { x: chartX, y: chartY } : null);
      }
    };

    load().catch((err: Error) => {
      if (active) {
        setStatsError(err.message);
      }
    });

    return () => {
      active = false;
    };
  }, [runs, file, column, metric, limit]);

  const chartOptions = useMemo(
    () => ({
      height: 220,
      ms: 1,
      scales: {
        x: { time: true },
      },
      series: [
        {},
        {
          label: metric.toUpperCase(),
          stroke: palette[0],
          width: 2,
        },
      ],
      axes: [
        { stroke: getCssVar("--muted", "#6a6f73") },
        { stroke: getCssVar("--muted", "#6a6f73") },
      ],
    }),
    [metric, theme],
  );

  if (loading) {
    return (
      <Panel>
        <EmptyState>Loading runs…</EmptyState>
      </Panel>
    );
  }

  if (error) {
    return (
      <Panel>
        <EmptyState>{error}</EmptyState>
      </Panel>
    );
  }

  if (!runs || runs.length === 0) {
    return (
      <Panel>
        <EmptyState>No runs yet. Run a benchmark and reload.</EmptyState>
      </Panel>
    );
  }

  if (!csvFiles.length) {
    return (
      <Panel>
        <EmptyState>No CSV outputs found in the latest run.</EmptyState>
      </Panel>
    );
  }

  if (!columnOptions.length) {
    return (
      <Panel>
        <EmptyState>No numeric columns found for {file}.</EmptyState>
      </Panel>
    );
  }

  return (
    <div className={layout.stack}>
      <Panel>
        <div className={layout.stack}>
          <div className={layout.row}>
            <Tag>Aggregate</Tag>
            <h2>Run Overview</h2>
          </div>
          <div className={layout.controls}>
            <label>
              Output
              <select value={file} onChange={(event) => setFile(event.target.value)}>
                {csvFiles.map((csvFile) => (
                  <option key={csvFile} value={csvFile}>
                    {csvFile}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Column
              <select value={column} onChange={(event) => setColumn(event.target.value)}>
                {columnOptions.map((col) => (
                  <option key={col} value={col}>
                    {col}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Metric
              <select value={metric} onChange={(event) => setMetric(event.target.value as Metric)}>
                {(["mean", "median", "p95", "p99"] as Metric[]).map((value) => (
                  <option key={value} value={value}>
                    {value.toUpperCase()}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Runs
              <input
                type="number"
                min={1}
                max={runs.length}
                value={limit}
                onChange={(event) => {
                  const parsed = Number(event.target.value);
                  setLimit(Number.isFinite(parsed) ? Math.max(1, parsed) : limit);
                }}
              />
            </label>
          </div>
          <Panel soft>
            {chartData ? (
              <Chart data={[chartData.x, chartData.y]} options={chartOptions} small />
            ) : (
              <EmptyState>No chart data.</EmptyState>
            )}
          </Panel>
        </div>
      </Panel>

      <Panel>
        <h3>Summary</h3>
        <div>
          <Table>
            <thead>
              <tr>
                <th>Run</th>
                <th>Start</th>
                <th>Count</th>
                <th>Min</th>
                <th>Mean</th>
                <th>Median</th>
                <th>P95</th>
                <th>P99</th>
              </tr>
            </thead>
            <tbody>
              {statsError ? (
                <tr>
                  <td colSpan={8}>{statsError}</td>
                </tr>
              ) : rows.length ? (
                rows.map((run) => {
                  const stats = statsMap.get(run.id);
                  if (!stats) {
                    return null;
                  }
                  return (
                    <tr key={run.id}>
                      <td>
                        <Link to={`/run/${encodeURIComponent(run.id)}`}>Run {run.id}</Link>
                      </td>
                      <td>{formatDateTime(run.start_time)}</td>
                      <td>{stats.count}</td>
                      <td>{formatNumber(stats.min)}</td>
                      <td>{formatNumber(stats.mean)}</td>
                      <td>{formatNumber(stats.median)}</td>
                      <td>{formatNumber(stats.p95)}</td>
                      <td>{formatNumber(stats.p99)}</td>
                    </tr>
                  );
                })
              ) : (
                <tr>
                  <td colSpan={8}>No matching data.</td>
                </tr>
              )}
            </tbody>
          </Table>
        </div>
      </Panel>
    </div>
  );
}
