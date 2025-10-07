default:
    @just --list

build:
    go build -o benchctl ./cmd/benchctl

schema:
    go run ./cmd/schema

compose-up:
    docker compose -f ./test/compose.yaml up -d

compose-down:
    docker compose -f ./test/compose.yaml down

integration-test:
    go test -tags=integration ./test/...

unit-test:
    go test -tags=unit ./internal/...

test:
    just integration-test
    just unit-test

# Example commands
example-local-container:
    @echo "Running local container example..."
    cd examples/local_container && ../../benchctl --config benchmark.yaml

example-local-container-clean:
    @echo "Cleaning up local container example..."
    cd examples/local_container && docker stop local-container-server 2>/dev/null || true
    cd examples/local_container && docker rm local-container-server 2>/dev/null || true
    cd examples/local_container && docker rmi local-container-server:latest 2>/dev/null || true
    cd examples/local_container && rm -rf results/