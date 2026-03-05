// Package views implements the reactive views engine with normalized refs architecture.
package views

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ViewDef describes a reactive view: its dependencies, parameter schema, and factory function.
type ViewDef struct {
	Key         string
	Tables      []TableDep
	ParamSchema map[string]string // param name -> type ("uuid", "string", "int")
	Factory     ViewFactory
}

// TableDep declares that a view depends on a specific DB table and columns.
type TableDep struct {
	Table   string   // e.g. "orders", "entities"
	Columns []string // columns that affect this view
	Filter  string   // optional: "kind=product" for entity table filtering
}

// ViewFactory executes the view query and returns normalized results.
type ViewFactory func(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, params map[string]any) (*ViewResult, error)

// ViewResult holds the normalized output of a view factory.
type ViewResult struct {
	Refs    []DataRef                    `json:"refs"`
	Tables  map[string]map[string]any    `json:"tables"`
	Version int64                        `json:"version"`
}

// DataRef is a pointer to a specific row in the normalized data store.
type DataRef struct {
	Table  string   `json:"table"`
	ID     string   `json:"id"`
	Fields []string `json:"fields"`
}

// ChangeEvent is produced by triggers from types.Event to signal which data changed.
type ChangeEvent struct {
	Table          string
	RowID          string
	ChangedColumns []string
	OrganizationID    uuid.UUID
}
