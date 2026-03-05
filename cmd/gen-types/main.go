package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/FrankFMY/arcana"
	"github.com/bizengine/engine/internal/graphs"
)

type noopTransport struct{}

func (noopTransport) SendToSeance(_ context.Context, _ string, _ arcana.Message) error { return nil }
func (noopTransport) SendToWorkspace(_ context.Context, _ string, _ arcana.Message) error {
	return nil
}
func (noopTransport) DisconnectSeance(_ context.Context, _ string) error { return nil }

type noopQuerier struct{}

func (noopQuerier) Query(_ context.Context, _ string, _ ...any) (arcana.Rows, error) {
	return &noopRows{}, nil
}

func (noopQuerier) QueryRow(_ context.Context, _ string, _ ...any) arcana.Row {
	return noopRow{}
}

type noopRows struct{}

func (noopRows) Next() bool          { return false }
func (noopRows) Scan(_ ...any) error { return nil }
func (noopRows) Close()              {}
func (noopRows) Err() error          { return nil }

type noopRow struct{}

func (noopRow) Scan(_ ...any) error { return nil }

func main() {
	output := flag.String("output", "./sdk/generated/", "output directory for generated .d.ts files")
	flag.Parse()

	if err := run(*output); err != nil {
		fmt.Fprintf(os.Stderr, "gen-types: %v\n", err)
		os.Exit(1)
	}
}

func run(outputDir string) error {
	engine := arcana.New(arcana.Config{
		Pool:       noopQuerier{},
		Transport:  noopTransport{},
		GCInterval: time.Hour,
	})

	graphs.RegisterAll(engine)

	registry := engine.Registry()

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	tablesPath := filepath.Join(outputDir, "tables.d.ts")
	if err := writeFile(tablesPath, func(f *os.File) error {
		return arcana.GenerateTables(f, registry)
	}); err != nil {
		return fmt.Errorf("generate tables: %w", err)
	}
	fmt.Printf("wrote %s\n", tablesPath)

	viewsPath := filepath.Join(outputDir, "views.d.ts")
	if err := writeFile(viewsPath, func(f *os.File) error {
		return arcana.GenerateViews(f, registry)
	}); err != nil {
		return fmt.Errorf("generate views: %w", err)
	}
	fmt.Printf("wrote %s\n", viewsPath)

	fmt.Printf("generated types for %d graphs\n", registry.GraphCount())
	return nil
}

func writeFile(path string, fn func(*os.File) error) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return fn(f)
}
