import { readdir, readFile, stat, writeFile } from "node:fs/promises";
import path from "node:path";
import { gzipSync } from "node:zlib";

const distDir = path.resolve(import.meta.dir, "..", "..", "internal", "uiassets", "dist");

async function walkDir(dir: string): Promise<string[]> {
  const entries = await readdir(dir, { withFileTypes: true });
  const files: string[] = [];
  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      files.push(...(await walkDir(fullPath)));
      continue;
    }
    files.push(fullPath);
  }
  return files;
}

async function gzipFile(filePath: string) {
  if (filePath.endsWith(".gz")) {
    return;
  }
  const fileStat = await stat(filePath);
  if (!fileStat.isFile()) {
    return;
  }
  const buffer = await readFile(filePath);
  const gzipped = gzipSync(buffer, { level: 9 });
  await writeFile(`${filePath}.gz`, gzipped);
}

async function main() {
  const files = await walkDir(distDir);
  await Promise.all(files.map((file) => gzipFile(file)));
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
