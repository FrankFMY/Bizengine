//go:build ignore
// +build ignore

// Command gen_view_types generates TypeScript types for all registered Arcana graphs.
// Run: go run scripts/gen_view_types.go
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/FrankFMY/arcana"

	"github.com/bizengine/engine/internal/graphs"
)

func main() {
	outputDir := "./sdk/generated"

	engine := arcana.New(arcana.Config{})

	graphs.RegisterAll(engine)
	registry := engine.Registry()

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		log.Fatalf("create output dir: %v", err)
	}

	tablesPath := filepath.Join(outputDir, "tables.d.ts")
	tablesFile, err := os.Create(tablesPath)
	if err != nil {
		log.Fatalf("create tables.d.ts: %v", err)
	}
	defer tablesFile.Close()

	if err := arcana.GenerateTables(tablesFile, registry); err != nil {
		log.Fatalf("generate tables: %v", err)
	}
	fmt.Printf("Generated %s\n", tablesPath)

	viewsPath := filepath.Join(outputDir, "views.d.ts")
	viewsFile, err := os.Create(viewsPath)
	if err != nil {
		log.Fatalf("create views.d.ts: %v", err)
	}
	defer viewsFile.Close()

	if err := arcana.GenerateViews(viewsFile, registry); err != nil {
		log.Fatalf("generate views: %v", err)
	}
	fmt.Printf("Generated %s\n", viewsPath)
}
