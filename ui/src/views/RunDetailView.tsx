import { useEffect, useMemo, useState } from "react";
import type uPlot from "uplot";
import Panel from "../components/Panel";
import Tag from "../components/Tag";
import Chart from "../components/Chart";
import EmptyState from "../components/EmptyState";
import { Pill, PillList } from "../components/Pill";
import layout from "../styles/layout.module.css";
import styles from "./RunDetailView.module.css";
import {
  applyRollingMean,
  buildSchemaMap,
  getColumnsFromSchema,
  isNumericColumn,
  isTimestampColumn,
  parseNumeric,
  parseTimestamp,
} from "../lib/data";
import { formatDateTime, formatDuration } from "../lib/format";
import { fetchCsv, useRunDetail } from "../lib/store";
import { getCssVar } from "../lib/dom";
import type { DataSchema } from "../types";

type RunDetailViewProps = {
  runId: string;
};

export default function RunDetailView({ runId }: RunDetailViewProps) {
  const { detail, error, loading } = useRunDetail(runId);
  const [csvControls, setCsvControls] = useState<Record<string, CsvControls>>({});
  const [csvPlots, setCsvPlots] = useState<Record<string, CsvPlotState>>({});

  const csvFiles = useMemo(
    () => detail?.files.filter((file) => file.kind === "csv").map((file) => file.name) ?? [],
    [detail],
  );
  const imageFiles = useMemo(
    () => detail?.files.filter((file) => file.kind === "image").map((file) => file.name) ?? [],
    [detail],
  );
  const schemaMap = useMemo(() => buildSchemaMap(detail?.metadata), [detail]);

  useEffect(() => {
    if (!csvFiles.length) {
      setCsvControls({});
      return;
    }
    setCsvControls((prev) => {
      const next = { ...prev };
      let changed = false;

      const ensureOptions = (schema?: DataSchema) => {
        const columns = getColumnsFromSchema(schema);
        const numeric = columns.filter(isNumericColumn).map((col) => col.name);
        const time = columns.filter(isTimestampColumn).map((col) => col.name);
        const xOptions = [...time, ...numeric].filter((value, index, self) => self.indexOf(value) === index);
        return { xOptions, numeric };
      };

      csvFiles.forEach((file) => {
        const schema = schemaMap.get(file);
        const { xOptions, numeric } = ensureOptions(schema);
        const existing = next[file];
        const fallbackXAxis = xOptions[0] ?? "";
        const fallbackYAxis = numeric[0] ?? fallbackXAxis;

        if (!existing) {
          next[file] = {
            xAxis: fallbackXAxis,
            yAxis: fallbackYAxis,
            align: "relative",
            smooth: 1,
          };
          changed = true;
          return;
        }

        let xAxis = existing.xAxis;
        let yAxis = existing.yAxis;
        let align = existing.align;
        let smooth = existing.smooth;

        if (!xOptions.includes(xAxis)) {
          xAxis = fallbackXAxis;
          changed = true;
        }
        if (!numeric.includes(yAxis)) {
          yAxis = fallbackYAxis;
          changed = true;
        }
        if (!xAxis) {
          xAxis = fallbackXAxis;
        }
        if (!yAxis) {
          yAxis = fallbackYAxis;
        }
        if (align !== "absolute" && align !== "relative") {
          align = "relative";
          changed = true;
        }
        if (!Number.isFinite(smooth) || smooth < 1) {
          smooth = 1;
          changed = true;
        }

        next[file] = { xAxis, yAxis, align, smooth };
      });

      Object.keys(next).forEach((key) => {
        if (!csvFiles.includes(key)) {
          delete next[key];
          changed = true;
        }
      });

      return changed ? next : prev;
    });
  }, [csvFiles, schemaMap]);

  useEffect(() => {
    if (!detail || !csvFiles.length) {
      setCsvPlots({});
      return;
    }
    let active = true;
    const runStart = Date.parse(detail.metadata.start_time);

    const load = async () => {
      const entries = await Promise.all(
        csvFiles.map(async (file) => {
          const control = csvControls[file];
          const schema = schemaMap.get(file);
          const schemaMapByName = new Map(schema?.columns?.map((col) => [col.name, col]));
          const xCol = schemaMapByName.get(control?.xAxis ?? "");
          const xIsTime = xCol ? isTimestampColumn(xCol) : false;

          if (!control?.xAxis || !control?.yAxis) {
            return [file, { data: null, series: null, xIsTime } satisfies CsvPlotState] as const;
          }

          const csv = await fetchCsv(runId, file);
          const xValues: number[] = [];
          const yValues: (number | null)[] = [];

          for (const row of csv.rows) {
            const rawX = row[control.xAxis];
            const rawY = row[control.yAxis];
            if (rawX == null || rawY == null) {
              continue;
            }
            let xValue: number | null = null;
            if (xIsTime) {
              xValue = parseTimestamp(rawX, xCol?.format?.toLowerCase(), xCol?.unit?.toLowerCase());
              if (xValue !== null && control.align === "relative" && !Number.isNaN(runStart)) {
                xValue -= runStart;
              }
            } else {
              xValue = parseNumeric(rawX);
            }
            const yValue = parseNumeric(rawY);
            if (xValue === null || yValue === null) {
              continue;
            }
            xValues.push(xValue);
            yValues.push(yValue);
          }

          if (!xValues.length) {
            return [file, { data: null, series: null, xIsTime } satisfies CsvPlotState] as const;
          }

          const smoothed = applyRollingMean(yValues, control.smooth);
          const data: uPlot.AlignedData = [xValues, smoothed];
          const seriesOpts: uPlot.Series[] = [
            {},
            {
              label: `Run ${detail.metadata.run_id}`,
              stroke: "#1f9e8f",
              width: 2,
            },
          ];

          return [file, { data, series: seriesOpts, xIsTime } satisfies CsvPlotState] as const;
        }),
      );

      if (active) {
        const next: Record<string, CsvPlotState> = {};
        entries.forEach(([file, state]) => {
          next[file] = state;
        });
        setCsvPlots(next);
      }
    };

    load().catch((err: Error) => {
      if (active) {
        const next: Record<string, CsvPlotState> = {};
        csvFiles.forEach((file) => {
          next[file] = { data: null, series: null, xIsTime: false, error: err.message };
        });
        setCsvPlots(next);
      }
    });

    return () => {
      active = false;
    };
  }, [csvFiles, csvControls, detail, runId, schemaMap]);

  if (loading) {
    return (
      <Panel>
        <EmptyState>Loading run…</EmptyState>
      </Panel>
    );
  }

  if (error || !detail) {
    return (
      <Panel>
        <EmptyState>{error ?? "Run not found."}</EmptyState>
      </Panel>
    );
  }

  const meta = detail.metadata;

  return (
    <div className={layout.stack}>
      <Panel>
        <div className={layout.stack}>
          <div className={layout.row}>
            <Tag>Run {meta.run_id}</Tag>
            <h2>{meta.benchmark_name}</h2>
          </div>
          <div className={styles.metaGrid}>
            <div className={styles.metaCard}>
              <div className={styles.metaTitle}>Start</div>
              <div className={styles.metaValue}>{formatDateTime(meta.start_time)}</div>
            </div>
            <div className={styles.metaCard}>
              <div className={styles.metaTitle}>End</div>
              <div className={styles.metaValue}>{formatDateTime(meta.end_time)}</div>
            </div>
            <div className={styles.metaCard}>
              <div className={styles.metaTitle}>Duration</div>
              <div className={styles.metaValue}>{formatDuration(meta.start_time, meta.end_time)}</div>
            </div>
          </div>
          <PillList>
            {Object.entries(meta.custom ?? {}).map(([key, value]) => (
              <Pill key={key}>
                {key}: {value}
              </Pill>
            ))}
          </PillList>
        </div>
      </Panel>

      <Panel>
        <div className={layout.stack}>
          <div className={styles.sectionHeader}>
            <Tag>CSV</Tag>
            <h2>CSV outputs</h2>
          </div>
          {!csvFiles.length ? (
            <EmptyState>No CSV outputs found.</EmptyState>
          ) : (
            csvFiles.map((file) => {
              const control = csvControls[file];
              const schema = schemaMap.get(file);
              const columns = getColumnsFromSchema(schema);
              const numericCols = columns.filter(isNumericColumn).map((col) => col.name);
              const timeCols = columns.filter(isTimestampColumn).map((col) => col.name);
              const xOptions = [...timeCols, ...numericCols].filter(
                (value, index, self) => self.indexOf(value) === index,
              );
              const plotState = csvPlots[file];
              const options = plotState?.series
                ? {
                    height: 360,
                    ms: 1,
                    scales: {
                      x: { time: plotState.xIsTime && control?.align === "absolute" },
                    },
                    axes: [
                      {
                        stroke: getCssVar("--muted", "#6a6f73"),
                        values:
                          plotState.xIsTime && control?.align === "relative"
                            ? (_u: uPlot, vals: number[]) =>
                                vals.map((value) => `${(value / 1000).toFixed(1)}s`)
                            : undefined,
                      },
                      { stroke: getCssVar("--muted", "#6a6f73") },
                    ],
                    series: plotState.series,
                  }
                : null;

              return (
                <Panel key={file} soft>
                  <div className={layout.stack}>
                    <div className={styles.csvHeader}>
                      <div>
                        <h3>{file}</h3>
                        <p className={styles.csvSub}>Plotting run {meta.run_id}.</p>
                      </div>
                    </div>
                    <div className={layout.controls}>
                      <label>
                        X axis
                        <select
                          value={control?.xAxis ?? ""}
                          onChange={(event) =>
                            setCsvControls((prev) => ({
                              ...prev,
                              [file]: {
                                ...(prev[file] ?? {
                                  xAxis: "",
                                  yAxis: "",
                                  align: "relative",
                                  smooth: 1,
                                }),
                                xAxis: event.target.value,
                              },
                            }))
                          }
                        >
                          {xOptions.map((value) => (
                            <option key={value} value={value}>
                              {value}
                            </option>
                          ))}
                        </select>
                      </label>
                      <label>
                        Y axis
                        <select
                          value={control?.yAxis ?? ""}
                          onChange={(event) =>
                            setCsvControls((prev) => ({
                              ...prev,
                              [file]: {
                                ...(prev[file] ?? {
                                  xAxis: "",
                                  yAxis: "",
                                  align: "relative",
                                  smooth: 1,
                                }),
                                yAxis: event.target.value,
                              },
                            }))
                          }
                        >
                          {numericCols.map((value) => (
                            <option key={value} value={value}>
                              {value}
                            </option>
                          ))}
                        </select>
                      </label>
                      <label>
                        Align
                        <select
                          value={control?.align ?? "relative"}
                          onChange={(event) =>
                            setCsvControls((prev) => ({
                              ...prev,
                              [file]: {
                                ...(prev[file] ?? {
                                  xAxis: "",
                                  yAxis: "",
                                  align: "relative",
                                  smooth: 1,
                                }),
                                align: event.target.value as AlignMode,
                              },
                            }))
                          }
                        >
                          <option value="relative">Relative</option>
                          <option value="absolute">Absolute</option>
                        </select>
                      </label>
                      <label>
                        Smooth
                        <input
                          type="number"
                          min={1}
                          max={50}
                          value={control?.smooth ?? 1}
                          onChange={(event) => {
                            const val = Number(event.target.value);
                            setCsvControls((prev) => ({
                              ...prev,
                              [file]: {
                                ...(prev[file] ?? {
                                  xAxis: "",
                                  yAxis: "",
                                  align: "relative",
                                  smooth: 1,
                                }),
                                smooth: Number.isFinite(val) ? Math.max(1, val) : 1,
                              },
                            }));
                          }}
                        />
                      </label>
                    </div>
                    {plotState?.error ? (
                      <EmptyState>{plotState.error}</EmptyState>
                    ) : plotState?.data && options ? (
                      <Chart data={plotState.data} options={options} />
                    ) : (
                      <EmptyState>No data found for this file.</EmptyState>
                    )}
                  </div>
                </Panel>
              );
            })
          )}
        </div>
      </Panel>

      <Panel>
        <div className={layout.stack}>
          <div className={styles.sectionHeader}>
            <Tag>Images</Tag>
            <h2>Image outputs</h2>
          </div>
          {!imageFiles.length ? (
            <EmptyState>No image outputs found.</EmptyState>
          ) : (
            <div className={styles.imageGrid}>
              {imageFiles.map((file) => (
                <div key={file} className={styles.imageCard}>
                  <div className={styles.imageLabel}>{file}</div>
                  <a
                    className={styles.imageLink}
                    href={`/api/runs/${encodeURIComponent(runId)}/files/${encodeURIComponent(file)}`}
                    target="_blank"
                    rel="noreferrer"
                  >
                    <div className={styles.imageFrame}>
                      <img
                        src={`/api/runs/${encodeURIComponent(runId)}/files/${encodeURIComponent(file)}`}
                        alt={`${file} for run ${runId}`}
                      />
                    </div>
                  </a>
                </div>
              ))}
            </div>
          )}
        </div>
      </Panel>
    </div>
  );
}

type CsvControls = {
  xAxis: string;
  yAxis: string;
  align: AlignMode;
  smooth: number;
};

type CsvPlotState = {
  data: uPlot.AlignedData | null;
  series: uPlot.Series[] | null;
  xIsTime: boolean;
  error?: string;
};

type AlignMode = "absolute" | "relative";
