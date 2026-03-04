package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()

	defOrders := &ViewDef{
		Key: "orders_list",
		Tables: []TableDep{
			{Table: "orders", Columns: []string{"status", "total"}},
			{Table: "entities", Columns: []string{"name"}, Filter: "kind=order"},
		},
	}
	defProducts := &ViewDef{
		Key: "products_list",
		Tables: []TableDep{
			{Table: "catalog_products", Columns: []string{"name", "price"}},
			{Table: "entities", Columns: []string{"name"}, Filter: "kind=product"},
		},
	}
	defStock := &ViewDef{
		Key: "stock_levels",
		Tables: []TableDep{
			{Table: "stock_levels", Columns: []string{"quantity"}},
		},
	}

	r.Register(defOrders)
	r.Register(defProducts)
	r.Register(defStock)

	// Get by key
	got, ok := r.Get("orders_list")
	require.True(t, ok)
	assert.Equal(t, "orders_list", got.Key)

	_, ok = r.Get("nonexistent")
	assert.False(t, ok)

	// GetByTable: "entities" should return orders_list and products_list
	byEntities := r.GetByTable("entities")
	assert.Len(t, byEntities, 2)
	keys := map[string]bool{}
	for _, d := range byEntities {
		keys[d.Key] = true
	}
	assert.True(t, keys["orders_list"])
	assert.True(t, keys["products_list"])

	// GetByTable: "stock_levels" should return only stock_levels
	byStock := r.GetByTable("stock_levels")
	assert.Len(t, byStock, 1)
	assert.Equal(t, "stock_levels", byStock[0].Key)

	// GetByTable: unknown table returns nil
	assert.Nil(t, r.GetByTable("unknown"))

	// All returns all 3
	assert.Len(t, r.All(), 3)
}
