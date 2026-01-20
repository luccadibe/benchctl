set dotenv-load := true

default:
    @just --list

build:
    cd ui && bun run build
    go build -o benchctl ./cmd/benchctl

schema:
    go run ./cmd/schema 2> cmd/schema/schema.json

compose-up *FLAGS:
    docker compose -f ./test/compose.yaml up -d {{FLAGS}}

compose-down *FLAGS:
    docker compose -f ./test/compose.yaml down {{FLAGS}}

integration-test:
    go test -tags=integration ./test/...

unit-test:
    go test -tags=unit ./internal/...

test:
    just unit-test
    just compose-up 2>/dev/null
    sleep 2
    just integration-test
    just compose-down 2>/dev/null


# Example commands
example-local-container:
    @echo "Running local container example..."
    cd examples/local_container && ../../benchctl --config benchmark.yaml run --metadata "branch"="main" --metadata "commit"="example-run"

example-local-container-ui:
    @echo "Running local container example UI..."
    cd examples/local_container && ../../benchctl --config benchmark.yaml ui

example-local-container-clean:
    @echo "Cleaning up local container example..."
    cd examples/local_container && docker stop local-container-server 2>/dev/null || true
    cd examples/local_container && docker rm local-container-server 2>/dev/null || true
    cd examples/local_container && docker rmi local-container-server:latest 2>/dev/null || true
    cd examples/local_container && rm -rf results/
example-local-container-inspect run-id:
    @echo "Inspecting local container example..."
    cd examples/local_container && ../../benchctl --config benchmark.yaml inspect {{run-id}}
example-local-container-edit run-id *FLAGS:
    @echo "Editing local container example..."
    cd examples/local_container && ../../benchctl --config benchmark.yaml edit {{run-id}} {{FLAGS}}
example-local-container-compare run-id1 run-id2:
    @echo "Comparing local container example..."
    cd examples/local_container && ../../benchctl --config benchmark.yaml compare {{run-id1}} {{run-id2}}

tag version:
    git tag -a v{{version}} -m "Release {{version}}"
    git push origin v{{version}}

release:
    goreleaser release --clean
