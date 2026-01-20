import { useEffect, useState } from "react";
import Papa from "papaparse";
import { api, apiText } from "./api";
import type { CsvData, RunDetail, RunSummary } from "../types";

const runsCache: { data: RunSummary[] | null; promise: Promise<RunSummary[]> | null } = {
  data: null,
  promise: null,
};

const runDetailCache = new Map<string, RunDetail>();
const runDetailPromises = new Map<string, Promise<RunDetail>>();

const csvCache = new Map<string, CsvData>();
const csvPromises = new Map<string, Promise<CsvData>>();

async function fetchRuns() {
  if (runsCache.data) {
    return runsCache.data;
  }
  if (!runsCache.promise) {
    runsCache.promise = api<RunSummary[]>("/api/runs");
  }
  const data = await runsCache.promise;
  runsCache.data = data;
  return data;
}

export async function fetchRunDetail(id: string) {
  if (runDetailCache.has(id)) {
    return runDetailCache.get(id)!;
  }
  if (!runDetailPromises.has(id)) {
    runDetailPromises.set(id, api<RunDetail>(`/api/runs/${encodeURIComponent(id)}`));
  }
  const detail = await runDetailPromises.get(id)!;
  runDetailCache.set(id, detail);
  return detail;
}

export async function fetchCsv(runId: string, fileName: string) {
  const key = `${runId}:${fileName}`;
  if (csvCache.has(key)) {
    return csvCache.get(key)!;
  }
  if (!csvPromises.has(key)) {
    const promise = apiText(
      `/api/runs/${encodeURIComponent(runId)}/files/${encodeURIComponent(fileName)}`,
    ).then((text) => {
      const parsed = Papa.parse<Record<string, string>>(text, {
        header: true,
        skipEmptyLines: true,
      });
      return {
        fields: parsed.meta.fields ?? [],
        rows: parsed.data.filter((row) => row && Object.keys(row).length > 0),
      } as CsvData;
    });
    csvPromises.set(key, promise);
  }
  const data = await csvPromises.get(key)!;
  csvCache.set(key, data);
  return data;
}

export function useRuns() {
  const [runs, setRuns] = useState<RunSummary[] | null>(runsCache.data);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    fetchRuns()
      .then((data) => {
        if (active) {
          setRuns(data);
        }
      })
      .catch((err: Error) => {
        if (active) {
          setError(err.message);
        }
      });
    return () => {
      active = false;
    };
  }, []);

  return { runs, error, loading: !runs && !error };
}

export function useRunDetail(runId: string | null) {
  const [detail, setDetail] = useState<RunDetail | null>(
    runId ? runDetailCache.get(runId) ?? null : null,
  );
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!runId) {
      setDetail(null);
      return;
    }
    setError(null);
    let active = true;
    fetchRunDetail(runId)
      .then((data) => {
        if (active) {
          setDetail(data);
        }
      })
      .catch((err: Error) => {
        if (active) {
          setError(err.message);
        }
      });
    return () => {
      active = false;
    };
  }, [runId]);

  return { detail, error, loading: !!runId && !detail && !error };
}

export function useCsv(runId: string | null, fileName: string | null) {
  const key = runId && fileName ? `${runId}:${fileName}` : null;
  const [csv, setCsv] = useState<CsvData | null>(key ? csvCache.get(key) ?? null : null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!runId || !fileName) {
      setCsv(null);
      return;
    }
    setError(null);
    let active = true;
    fetchCsv(runId, fileName)
      .then((data) => {
        if (active) {
          setCsv(data);
        }
      })
      .catch((err: Error) => {
        if (active) {
          setError(err.message);
        }
      });
    return () => {
      active = false;
    };
  }, [runId, fileName]);

  return { csv, error, loading: !!runId && !!fileName && !csv && !error };
}
