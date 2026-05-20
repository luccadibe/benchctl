# Benchctl Agent Guide

This file describes how to build, test, and work in this repo.
It is intended for agentic coding assistants.

## Repo layout
- `cmd/benchctl`: CLI entrypoint.
- `internal/`: core Go packages.
- `test/`: integration tests and docker compose.
- `examples/`: demo configs and scripts.

## Build commands
- `just build`: build Go binary (writes `./benchctl`).
- `go build -o benchctl ./cmd/benchctl`: build Go binary only.

## Lint / format
- No dedicated linter configured.
- Go formatting uses `gofmt` (or `go fmt ./...`).
- Keep import groups gofmt-style (std lib, blank line, external).

## Test commands
- `just test`: run integration + unit tests.
- `just unit-test`: `go test -tags=unit ./internal/...`.
- `just integration-test`: `go test -tags=integration ./test/...`.

### Single test (unit)
- `go test -tags=unit ./internal/... -run '^TestName$'`
- `go test -tags=unit ./internal/config -run '^TestLoadConfig_Success$'`

### Single test (integration)
- `go test -tags=integration ./test/... -run '^TestSSHClientConnection$'`
- `go test -tags=integration ./test -run '^TestWorkflowSimple$'`

### Integration test setup
- `just compose-up` to start SSH containers.
- `just compose-down` to stop containers.
- SSH keys live in `test/testdata/ssh/` and are gitignored.
- See `test/README.md` for key regeneration steps.


## Go code style
- Use `gofmt` formatting and standard Go idioms.
- Prefer small, focused functions with clear names.
- Use `camelCase` for locals and `PascalCase` for exported types.
- Keep initialisms capitalized (`ID`, `URL`, `SSH`).
- Use struct tags in config/types for `yaml`/`json` as shown.
- Keep package-level comments concise and meaningful.
- Avoid one-letter names except in tight loops.
- Keep helper functions private unless needed externally.
- Prefer table-driven tests with `t.Run`.
- Use slices/maps with preallocation when sizes are known.
- Keep `internal/` packages decoupled; avoid cross-package cycles.

## Go imports
- Group imports: stdlib, blank line, third-party/internal.
- Avoid unused imports; let `gofmt`/`go vet` catch issues.
- Use blank identifier imports only for side effects (e.g. `//go:embed`).

## Error handling
- Return errors instead of panicking.
- Wrap errors with context: `fmt.Errorf("action: %w", err)`.
- Keep error strings lowercase, no trailing punctuation.
- Validate inputs early and return fast on failure.
- Avoid swallowing errors; if intentionally ignored, add a reason.

## Logging and output
- The CLI writes user-visible output via `fmt`/writers.
- Use existing logging patterns (see `internal/logging` if present).
- Keep output stable; CLI behavior is part of API surface.

## Config/schema rules
- Config types in `internal/config` are source of truth.
- Keep YAML/JSON tags in sync when adding fields.
- Validation errors should be aggregated when possible.
- Default values should be set in validation or constructor helpers, not ad hoc during execution.

## Tests
- Unit tests live under `internal/`.
- Integration tests live under `test/` and use Docker SSH containers.
- Use `_test.go` file naming and `TestXxx` functions.
- Prefer deterministic tests; avoid network unless required.

## Shell scripts / examples
- Scripts in `examples/` are user-facing; keep them readable.
- Use `set -e` only when existing scripts do.
- Keep paths relative to example directories.

## Docs
- `README.md` is user-facing; update only when features change.
- `docs/` contains detailed docs; match current behavior.

## Commit hygiene (for agents)
- Do not commit unless explicitly asked.
- Keep changes minimal and focused.

## Quick reference
- Build CLI: `go build -o benchctl ./cmd/benchctl`
- Run all tests: `just test`
- Unit tests: `just unit-test`
- Integration tests: `just compose-up && just integration-test && just compose-down`
- Single unit test: `go test -tags=unit ./internal/... -run '^TestName$'`
- Single integration test: `go test -tags=integration ./test/... -run '^TestName$'`

## Safety
- Keep vendor-like files intact.

## When adding new packages
- Update `go.mod`/`go.sum` via `go mod tidy`.
- Avoid adding heavy dependencies without justification.
- Prefer stdlib or existing deps.

## When editing configs
- Preserve YAML key naming conventions (snake_case).
- Ensure schema changes are reflected in config validation.

## When adding CLI flags
- Update `cmd/benchctl` command wiring.
- Keep help text concise and descriptive.
- Ensure defaults align with config validation.

## Known tags
- Unit tests use `-tags=unit`.
- Integration tests use `-tags=integration`.

## End
