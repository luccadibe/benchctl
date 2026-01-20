import { useEffect, useMemo, useState } from "react";
import type uPlot from "uplot";
import Panel from "../components/Panel";
import Tag from "../components/Tag";
import Chart from "../components/Chart";
import EmptyState from "../components/EmptyState";
import Table from "../components/Table";
import Preview from "../components/Preview";
import layout from "../styles/layout.module.css";
import styles from "./CompareView.module.css";
import {
  applyRollingMean,
  buildSchemaMap,
  getColumnsFromSchema,
  isNumericColumn,
  isTimestampColumn,
  parseNumeric,
  parseTimestamp,
} from "../lib/data";
import { formatDateTime } from "../lib/format";
import { fetchCsv, fetchRunDetail } from "../lib/store";
import { getCssVar } from "../lib/dom";
import type { DataSchema, RunSummary } from "../types";

type CompareViewProps = {
  runs: RunSummary[] | null;
  loading: boolean;
  error: string | null;
};

type AlignMode = "absolute" | "relative";

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

const palette = ["#1f9e8f", "#f0a23b", "#2b7db6", "#e06c75", "#3c9d78", "#c19a6b"];

export default function CompareView({ runs, loading, error }: CompareViewProps) {
  const [selectedRuns, setSelectedRuns] = useState<string[]>([]);
  const [search, setSearch] = useState("");
  const [initialized, setInitialized] = useState(false);
  const [latestDetail, setLatestDetail] = useState<Awaited<ReturnType<typeof fetchRunDetail>> | null>(
    null,
  );
  const [csvControls, setCsvControls] = useState<Record<string, CsvControls>>({});
  const [csvPlots, setCsvPlots] = useState<Record<string, CsvPlotState>>({});
  const [metaTable, setMetaTable] = useState<{
    runIds: string[];
    keys: string[];
    values: Record<string, Record<string, string | null>>;
  } | null>(null);

  const latestRun = runs?.[0];

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
      .catch(() => {
        if (active) {
          setLatestDetail(null);
        }
      });
    return () => {
      active = false;
    };
  }, [latestRun]);

  const csvFiles = useMemo(
    () => latestDetail?.files.filter((f) => f.kind === "csv").map((f) => f.name) ?? [],
    [latestDetail],
  );
  const imageFiles = useMemo(
    () => latestDetail?.files.filter((f) => f.kind === "image").map((f) => f.name) ?? [],
    [latestDetail],
  );
  const schemaMap = useMemo(() => buildSchemaMap(latestDetail?.metadata), [latestDetail]);

  useEffect(() => {
    if (initialized || !runs?.length) {
      return;
    }
    const params = new URLSearchParams(window.location.search);
    const preRuns = params.get("runs");
    let initial: string[] = [];
    if (preRuns) {
      const ids = preRuns
        .split(",")
        .map((id) => id.trim())
        .filter(Boolean);
      initial = ids.filter((id) => runs.some((run) => run.id === id));
    }
    if (!initial.length) {
      initial = runs.slice(0, 3).map((run) => run.id);
    }
    setSelectedRuns(initial);
    setInitialized(true);
  }, [initialized, runs]);

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
    if (!csvFiles.length || !selectedRuns.length) {
      setCsvPlots({});
      return;
    }
    let active = true;

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

          const seriesData: { runId: string; x: number[]; y: (number | null)[] }[] = [];

          for (const [idx, runId] of selectedRuns.entries()) {
            const detail = await fetchRunDetail(runId);
            const fileInfo = detail.files.find((f) => f.name === file);
            if (!fileInfo) {
              continue;
            }
            const csv = await fetchCsv(runId, file);
            const xValues: number[] = [];
            const yValues: (number | null)[] = [];
            const runStart = Date.parse(detail.metadata.start_time);

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

            const smoothed = applyRollingMean(yValues, control.smooth);
            seriesData.push({ runId, x: xValues, y: smoothed });
            if (idx >= palette.length) {
              palette.push(`#${Math.floor(Math.random() * 0xffffff).toString(16).padStart(6, "0")}`);
            }
          }

          if (!seriesData.length) {
            return [file, { data: null, series: null, xIsTime } satisfies CsvPlotState] as const;
          }

          const xUnion = Array.from(new Set(seriesData.flatMap((series) => series.x))).sort((a, b) => a - b);
          const alignedSeries = seriesData.map((series) => {
            const map = new Map<number, number | null>();
            series.x.forEach((x, idx) => {
              map.set(x, series.y[idx] ?? null);
            });
            return xUnion.map((x) => map.get(x) ?? null);
          });

          const data: uPlot.AlignedData = [xUnion, ...alignedSeries];
          const seriesOpts: uPlot.Series[] = [
            {},
            ...seriesData.map((series, idx) => ({
              label: `Run ${series.runId}`,
              stroke: palette[idx % palette.length],
              width: 2,
            })),
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
  }, [csvFiles, selectedRuns, csvControls, schemaMap]);

  useEffect(() => {
    if (!selectedRuns.length) {
      setMetaTable(null);
      return;
    }
    let active = true;
    const load = async () => {
      const details = await Promise.all(selectedRuns.map((id) => fetchRunDetail(id)));
      const keys = Array.from(
        new Set(details.flatMap((detail) => Object.keys(detail.metadata.custom ?? {}))),
      ).sort();
      const values: Record<string, Record<string, string | null>> = {};
      keys.forEach((key) => {
        values[key] = {};
        details.forEach((detail) => {
          values[key][detail.metadata.run_id] = detail.metadata.custom?.[key] ?? null;
        });
      });
      if (active) {
        setMetaTable({ runIds: details.map((detail) => detail.metadata.run_id), keys, values });
      }
    };
    load().catch(() => {
      if (active) {
        setMetaTable(null);
      }
    });
    return () => {
      active = false;
    };
  }, [selectedRuns]);

  const matchRuns = useMemo(() => {
    if (!runs) {
      return [];
    }
    const query = search.trim().toLowerCase();
    return runs
      .filter((run) => {
        if (!query) return true;
        const haystack = [
          run.id,
          run.benchmark_name,
          ...Object.entries(run.custom ?? {}).flat(),
        ]
          .join(" ")
          .toLowerCase();
        return haystack.includes(query);
      })
      .slice(0, 6);
  }, [runs, search]);

  const orderedSelectedRuns = useMemo(() => {
    if (!runs) {
      return [];
    }
    const set = new Set(selectedRuns);
    return runs.filter((run) => set.has(run.id));
  }, [runs, selectedRuns]);

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

  return (
    <div className={layout.stack}>
      <Panel>
        <div className={styles.selector}>
          <div className={styles.selectorHeader}>
            <Tag>Runs</Tag>
            <h2>Run selection</h2>
          </div>
          <div className={styles.selectedRuns}>
            {orderedSelectedRuns.length ? (
              orderedSelectedRuns.map((run) => (
                <div key={run.id} className={styles.runChip}>
                  <span>Run {run.id}</span>
                  <button
                    type="button"
                    className={styles.runAction}
                    onClick={() => setSelectedRuns((prev) => prev.filter((id) => id !== run.id))}
                  >
                    ×
                  </button>
                </div>
              ))
            ) : (
              <div className={styles.mutedNote}>No runs selected.</div>
            )}
          </div>
          <div className={styles.searchRow}>
            <label>
              Find run
              <input
                type="text"
                placeholder="Search by id or metadata"
                value={search}
                onChange={(event) => setSearch(event.target.value)}
              />
            </label>
          </div>
          <div className={styles.runResults}>
            {matchRuns.length ? (
              matchRuns.map((run) => {
                const selected = selectedRuns.includes(run.id);
                return (
                  <div key={run.id} className={styles.runResult}>
                    <div>
                      <strong>Run {run.id}</strong>
                      <div className={styles.runSub}>{formatDateTime(run.start_time)}</div>
                    </div>
                    <button
                      type="button"
                      className={`${styles.runAction} ${selected ? styles.runActionMuted : styles.runActionPrimary}`}
                      onClick={() => {
                        setSelectedRuns((prev) =>
                          selected ? prev.filter((id) => id !== run.id) : [...prev, run.id],
                        );
                      }}
                    >
                      {selected ? "Remove" : "Add"}
                    </button>
                  </div>
                );
              })
            ) : (
              <div className={styles.mutedNote}>No matching runs.</div>
            )}
          </div>
        </div>
      </Panel>

      <Panel>
        <div className={layout.stack}>
          <div className={styles.sectionHeader}>
            <Tag>CSV</Tag>
            <h2>CSV comparisons</h2>
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
                        <p className={styles.csvSub}>Comparing {selectedRuns.length || 0} runs.</p>
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
                    {selectedRuns.length === 0 ? (
                      <EmptyState>Select at least one run.</EmptyState>
                    ) : plotState?.error ? (
                      <EmptyState>{plotState.error}</EmptyState>
                    ) : plotState?.data && options ? (
                      <Chart data={plotState.data} options={options} />
                    ) : (
                      <EmptyState>No data found for selected runs.</EmptyState>
                    )}
                  </div>
                </Panel>
              );
            })
          )}
        </div>
      </Panel>

      <Panel>
        <details className={styles.metaToggle}>
          <summary>Custom metadata comparison</summary>
          <Preview>
            {!selectedRuns.length ? (
              <EmptyState>Select at least one run.</EmptyState>
            ) : metaTable?.keys.length ? (
              <Table>
                <thead>
                  <tr>
                    <th>Key</th>
                    {metaTable.runIds.map((id) => (
                      <th key={id}>Run {id}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {metaTable.keys.map((key) => (
                    <tr key={key}>
                      <td>{key}</td>
                      {metaTable.runIds.map((id) => (
                        <td key={id}>{metaTable.values[key]?.[id] ?? "-"}</td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </Table>
            ) : (
              <EmptyState>No custom metadata found for selected runs.</EmptyState>
            )}
          </Preview>
        </details>
      </Panel>

      <Panel>
        <div className={layout.stack}>
          <div className={styles.sectionHeader}>
            <Tag>Images</Tag>
            <h2>Image comparisons</h2>
          </div>
          {!imageFiles.length ? (
            <EmptyState>No image outputs found.</EmptyState>
          ) : selectedRuns.length === 0 ? (
            <EmptyState>Select at least one run.</EmptyState>
          ) : (
            imageFiles.map((file) => (
              <div key={file} className={styles.imageSection}>
                <div className={styles.imageHeader}>
                  <h3>{file}</h3>
                </div>
                <div className={styles.imageGrid}>
                  {orderedSelectedRuns.map((run) => (
                    <div key={run.id} className={styles.imageCard}>
                      <div className={styles.imageLabel}>Run {run.id}</div>
                      <div className={styles.imageFrame}>
                        <img
                          src={`/api/runs/${encodeURIComponent(run.id)}/files/${encodeURIComponent(file)}`}
                          alt={`${file} for run ${run.id}`}
                        />
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            ))
          )}
        </div>
      </Panel>
    </div>
  );
}
