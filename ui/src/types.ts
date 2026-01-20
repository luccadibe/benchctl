export type RunSummary = {
  id: string;
  benchmark_name: string;
  start_time: string;
  end_time: string;
  custom?: Record<string, string>;
};

export type FileInfo = {
  name: string;
  ext: string;
  size: number;
  mtime: string;
  kind: "csv" | "image" | "other";
};

export type RunDetail = {
  metadata: RunMetadata;
  files: FileInfo[];
};

export type RunMetadata = {
  run_id: string;
  benchmark_name: string;
  start_time: string;
  end_time: string;
  custom?: Record<string, string>;
  config?: Config;
};

export type Config = {
  stages?: Stage[];
};

export type Stage = {
  outputs?: Output[];
};

export type Output = {
  name: string;
  remote_path: string;
  data_schema?: DataSchema;
};

export type DataSchema = {
  format: string;
  columns: DataColumn[];
};

export type DataColumn = {
  name: string;
  type: string;
  unit?: string;
  format?: string;
};

export type CsvData = {
  fields: string[];
  rows: Record<string, string>[];
};
