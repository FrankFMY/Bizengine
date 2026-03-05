//go:build integration

package postgres

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bizengine/engine/internal/core/entity"
	"github.com/bizengine/engine/internal/storage/postgres/testutil"
	"github.com/bizengine/engine/pkg/types"
)

func TestIntegrationEntityCRUD(t *testing.T) {
	pool := testutil.NewTestPool(t)
	repo := NewEntityRepo(pool)
	ctx := context.Background()

	uidStr, oidStr := testutil.SeedOrganization(t, pool)
	orgID := uuid.MustParse(oidStr.String())
	_ = uidStr

	// Create
	e := &types.Entity{
		OrganizationID: orgID,
		Kind:        "product",
		Name:        "Integration Widget",
		Meta:        json.RawMessage(`{"color":"blue"}`),
	}
	err := repo.Create(ctx, e)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, e.ID)
	assert.Equal(t, "active", e.Status)

	// GetByID
	got, err := repo.GetByID(ctx, orgID, e.ID)
	require.NoError(t, err)
	assert.Equal(t, "Integration Widget", got.Name)
	assert.Equal(t, "product", got.Kind)

	// Update
	got.Name = "Updated Widget"
	err = repo.Update(ctx, got)
	require.NoError(t, err)

	got2, err := repo.GetByID(ctx, orgID, e.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Widget", got2.Name)

	// List
	e2 := &types.Entity{OrganizationID: orgID, Kind: "vehicle", Name: "Truck"}
	require.NoError(t, repo.Create(ctx, e2))

	entities, total, err := repo.List(ctx, orgID, entity.ListFilter{})
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, entities, 2)

	// List with filter
	kind := "product"
	entities, total, err = repo.List(ctx, orgID, entity.ListFilter{Kind: &kind})
	require.NoError(t, err)
	assert.Equal(t, 1, total)

	// SoftDelete
	err = repo.SoftDelete(ctx, orgID, e.ID)
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, orgID, e.ID)
	require.Error(t, err)
}

func TestIntegrationEntityNotFound(t *testing.T) {
	pool := testutil.NewTestPool(t)
	repo := NewEntityRepo(pool)
	ctx := context.Background()

	_, oidStr := testutil.SeedOrganization(t, pool)
	orgID := uuid.MustParse(oidStr.String())

	_, err := repo.GetByID(ctx, orgID, uuid.New())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestIntegrationComponents(t *testing.T) {
	pool := testutil.NewTestPool(t)
	repo := NewEntityRepo(pool)
	ctx := context.Background()

	_, oidStr := testutil.SeedOrganization(t, pool)
	orgID := uuid.MustParse(oidStr.String())

	e := &types.Entity{OrganizationID: orgID, Kind: "vehicle", Name: "Car"}
	require.NoError(t, repo.Create(ctx, e))

	// SetComponent
	c := &types.Component{
		EntityID:    e.ID,
		OrganizationID: orgID,
		Type:        "geo",
		Data:        json.RawMessage(`{"lat":55.75,"lng":37.62}`),
	}
	err := repo.SetComponent(ctx, c)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, c.ID)
	assert.Equal(t, int64(1), c.Version)

	// GetComponent
	got, err := repo.GetComponent(ctx, orgID, e.ID, "geo")
	require.NoError(t, err)
	assert.Equal(t, "geo", got.Type)

	// Upsert — version increments
	c2 := &types.Component{
		EntityID:    e.ID,
		OrganizationID: orgID,
		Type:        "geo",
		Data:        json.RawMessage(`{"lat":56.0,"lng":38.0}`),
	}
	err = repo.SetComponent(ctx, c2)
	require.NoError(t, err)
	assert.Equal(t, int64(2), c2.Version)

	// ListComponents
	c3 := &types.Component{
		EntityID:    e.ID,
		OrganizationID: orgID,
		Type:        "speed",
		Data:        json.RawMessage(`{"value":60}`),
	}
	require.NoError(t, repo.SetComponent(ctx, c3))

	comps, err := repo.ListComponents(ctx, orgID, e.ID)
	require.NoError(t, err)
	assert.Len(t, comps, 2)

	// DeleteComponent
	err = repo.DeleteComponent(ctx, orgID, e.ID, "speed")
	require.NoError(t, err)

	comps, err = repo.ListComponents(ctx, orgID, e.ID)
	require.NoError(t, err)
	assert.Len(t, comps, 1)

	// DeleteComponent not found
	err = repo.DeleteComponent(ctx, orgID, e.ID, "nonexistent")
	require.Error(t, err)
}

func TestIntegrationEventStore(t *testing.T) {
	pool := testutil.NewTestPool(t)
	store := NewEventStore(pool)
	ctx := context.Background()

	_, oidStr := testutil.SeedOrganization(t, pool)
	orgID := uuid.MustParse(oidStr.String())
	entityID := uuid.New()

	// Append events
	for i := 0; i < 5; i++ {
		ev := types.Event{
			ID:          uuid.New(),
			OrganizationID: orgID,
			EntityID:    &entityID,
			Type:        "entity.updated",
			Data:        json.RawMessage(`{"i":` + string(rune('0'+i)) + `}`),
			Timestamp:   time.Now().Add(time.Duration(i) * time.Second),
			Version:     1,
		}
		require.NoError(t, store.Append(ctx, ev))
	}

	// Append event of different type
	ev2 := types.Event{
		ID:          uuid.New(),
		OrganizationID: orgID,
		EntityID:    &entityID,
		Type:        "entity.created",
		Data:        json.RawMessage(`{}`),
		Timestamp:   time.Now(),
		Version:     1,
	}
	require.NoError(t, store.Append(ctx, ev2))

	// GetByEntity
	events, err := store.GetByEntity(ctx, orgID, entityID, nil, 100)
	require.NoError(t, err)
	assert.Len(t, events, 6)

	// GetByOrganization
	events, total, err := store.GetByOrganization(ctx, orgID, 100, 0)
	require.NoError(t, err)
	assert.Equal(t, 6, total)
	assert.Len(t, events, 6)

	// GetByType
	events, err = store.GetByType(ctx, orgID, "entity.created", nil, 100)
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, "entity.created", events[0].Type)
}

func TestIntegrationOrganizationIsolation(t *testing.T) {
	pool := testutil.NewTestPool(t)
	repo := NewEntityRepo(pool)
	ctx := context.Background()

	_, oidStr1 := testutil.SeedOrganization(t, pool)
	orgID1 := uuid.MustParse(oidStr1.String())

	// Create second organization
	var uid2, oid2 string
	pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, full_name) VALUES ('test2@example.com', '$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ0123456', 'Test2') RETURNING id`,
	).Scan(&uid2)
	pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug, owner_id) VALUES ('Test WS 2', 'test-ws-2', $1) RETURNING id`, uid2,
	).Scan(&oid2)
	orgID2 := uuid.MustParse(oid2)

	// Create entity in ws1
	e1 := &types.Entity{OrganizationID: orgID1, Kind: "product", Name: "WS1 Product"}
	require.NoError(t, repo.Create(ctx, e1))

	// Create entity in ws2
	e2 := &types.Entity{OrganizationID: orgID2, Kind: "product", Name: "WS2 Product"}
	require.NoError(t, repo.Create(ctx, e2))

	// List should be isolated
	entities1, total1, err := repo.List(ctx, orgID1, entity.ListFilter{})
	require.NoError(t, err)
	assert.Equal(t, 1, total1)
	assert.Equal(t, "WS1 Product", entities1[0].Name)

	entities2, total2, err := repo.List(ctx, orgID2, entity.ListFilter{})
	require.NoError(t, err)
	assert.Equal(t, 1, total2)
	assert.Equal(t, "WS2 Product", entities2[0].Name)

	// Cross-organization GetByID should fail
	_, err = repo.GetByID(ctx, orgID1, e2.ID)
	require.Error(t, err)
}

func TestIntegrationEntitySearch(t *testing.T) {
	pool := testutil.NewTestPool(t)
	repo := NewEntityRepo(pool)
	ctx := context.Background()

	_, oidStr := testutil.SeedOrganization(t, pool)
	orgID := uuid.MustParse(oidStr.String())

	require.NoError(t, repo.Create(ctx, &types.Entity{OrganizationID: orgID, Kind: "product", Name: "Red Widget"}))
	require.NoError(t, repo.Create(ctx, &types.Entity{OrganizationID: orgID, Kind: "product", Name: "Blue Widget"}))
	require.NoError(t, repo.Create(ctx, &types.Entity{OrganizationID: orgID, Kind: "product", Name: "Green Gadget"}))

	search := "Widget"
	entities, total, err := repo.List(ctx, orgID, entity.ListFilter{Search: &search})
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, entities, 2)
}

func TestIntegrationTransactions(t *testing.T) {
	pool := testutil.NewTestPool(t)
	repo := NewEntityRepo(pool)
	ctx := context.Background()

	_, oidStr := testutil.SeedOrganization(t, pool)
	orgID := uuid.MustParse(oidStr.String())

	// WithTx commit
	e := &types.Entity{OrganizationID: orgID, Kind: "product", Name: "TxTest"}
	err := repo.WithTx(ctx, func(tx pgx.Tx) error {
		return repo.CreateTx(ctx, tx, e)
	})
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, orgID, e.ID)
	require.NoError(t, err)
	assert.Equal(t, "TxTest", got.Name)
}
