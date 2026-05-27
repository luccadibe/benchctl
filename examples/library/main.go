// Library example: define and run a benchmark entirely from Go.
//
// Usage:
//
//	go run .
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/luccadibe/benchctl/pkg/bench"
	"github.com/luccadibe/benchctl/pkg/run"
)

func main() {
	ctx := context.Background()

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
			bench.Stage("env-demo",
				bench.Host("local"),
				bench.Command("echo LOAD=$LOAD"),
			),
			bench.Stage("write-metrics",
				bench.Host("local"),
				bench.Command(`echo "timestamp_ms,value" > /tmp/library_metrics.csv
echo "$(date +%s%3N),1" >> /tmp/library_metrics.csv`),
				bench.Output("metrics",
					bench.RemotePath("/tmp/library_metrics.csv"),
				),
			),
		),
	)

	if err := b.Validate(); err != nil {
		log.Fatalf("validate: %v", err)
	}

	result, err := run.Run(ctx, b,
		run.WithMetadata("source", "examples/library"),
		run.WithEnv("LOAD", "42"),
		run.WithTimeout(5*time.Minute),
	)
	if err != nil {
		log.Fatalf("run: %v", err)
	}

	fmt.Printf("Run finished: id=%s dir=%s\n", result.RunID, result.RunDir)
	fmt.Println()
	fmt.Println(run.Inspect(result.RunDir, false))

	if err := run.Annotate(result.RunDir, map[string]string{
		"annotated_by": "library-example",
	}); err != nil {
		log.Fatalf("annotate: %v", err)
	}
	fmt.Println("Annotated run with annotated_by=library-example")

	// Re-run the same definition with a stage skipped (does not mutate the bench).
	result2, err := run.Run(ctx, b,
		run.WithMetadata("source", "examples/library"),
		run.Skip("write-metrics"),
	)
	if err != nil {
		log.Fatalf("run with skip: %v", err)
	}
	fmt.Printf("Second run (skipped write-metrics): id=%s dir=%s\n", result2.RunID, result2.RunDir)
}
