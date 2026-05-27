# Library Example

This example shows how to use benchctl as a Go library instead of the CLI. The full benchmark is defined in code — no YAML config file.

It covers:

- Defining a benchmark with `pkg/bench` (options pattern)
- Running it with `pkg/run`
- Runtime options: metadata, environment variables, skip, timeout
- Post-run operations: inspect and annotate

## Prerequisites

- Go 1.25.1+
- benchctl checked out locally (this example uses a `replace` directive in `go.mod`)

## Run

From this directory:

```bash
go run .
```

From the repo root:

```bash
just example-library
```

Results are written to `./results/<run-id>/`.

## API sketch

```go
b := bench.New("library-example",
    bench.WithResultsPath("./results"),
    bench.WithLogging(bench.LogInfo()),
    bench.WithGit(bench.RequireClean(false)),
    bench.WithHost("local", bench.Local()),
    bench.WithStages(
        bench.Stage("hello",
            bench.Host("local"),
            bench.Command("echo 'Hello from benchctl library'"),
        ),
    ),
)

result, err := run.Run(ctx, b,
    run.WithMetadata("source", "examples/library"),
    run.WithEnv("LOAD", "42"),
)
```

See [`main.go`](main.go) for the full example, including output collection and a second run with `run.Skip`.

## Packages

| Import | Purpose |
|--------|---------|
| `github.com/luccadibe/benchctl/pkg/bench` | Benchmark definition (hosts, stages, cases) |
| `github.com/luccadibe/benchctl/pkg/run` | Execute runs and operate on results |
