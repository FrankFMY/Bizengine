package settings

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bizengine/engine/internal/core/event"
	"github.com/bizengine/engine/pkg/types"
)

// --- mocks ---

type mockSettingsRepo struct {
	store map[uuid.UUID]*Settings
}

func newMockSettingsRepo() *mockSettingsRepo {
	return &mockSettingsRepo{store: make(map[uuid.UUID]*Settings)}
}

func (m *mockSettingsRepo) Get(_ context.Context, orgID uuid.UUID) (*Settings, error) {
	s, ok := m.store[orgID]
	if !ok {
		return nil, assert.AnError
	}
	cp := *s
	return &cp, nil
}

func (m *mockSettingsRepo) Upsert(_ context.Context, s *Settings) error {
	cp := *s
	m.store[s.OrganizationID] = &cp
	return nil
}

type mockBus struct {
	published []types.Event
}

func (b *mockBus) Publish(_ context.Context, ev types.Event) error {
	b.published = append(b.published, ev)
	return nil
}
func (b *mockBus) Subscribe(string, event.Subscriber)        {}
func (b *mockBus) SubscribePattern(string, event.Subscriber) {}
func (b *mockBus) SubscribeAll(event.Subscriber)             {}

// --- tests ---

func TestGetDefaults(t *testing.T) {
	orgID := uuid.New()
	ctx := context.Background()

	repo := newMockSettingsRepo()
	bus := &mockBus{}
	svc := NewService(repo, bus)

	s, err := svc.Get(ctx, orgID)
	require.NoError(t, err)
	assert.Equal(t, "RUB", s.Currency)
	assert.Equal(t, "Europe/Moscow", s.Timezone)
	assert.Equal(t, "ORD-{SEQ}", s.OrderNumberFormat)
}

func TestUpdateSettings(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	repo := newMockSettingsRepo()
	bus := &mockBus{}
	svc := NewService(repo, bus)

	svc.Get(ctx, orgID)

	usd := "USD"
	tz := "America/New_York"
	s, err := svc.Update(ctx, orgID, UpdateInput{
		Currency: &usd,
		Timezone: &tz,
	}, &actorID)

	require.NoError(t, err)
	assert.Equal(t, "USD", s.Currency)
	assert.Equal(t, "America/New_York", s.Timezone)
	assert.Equal(t, 2, s.Version)

	hasEvent := false
	for _, ev := range bus.published {
		if ev.Type == "settings.updated" {
			hasEvent = true
		}
	}
	assert.True(t, hasEvent)
}

func TestUpdateInvalidCurrency(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	repo := newMockSettingsRepo()
	bus := &mockBus{}
	svc := NewService(repo, bus)

	svc.Get(ctx, orgID)

	bad := "EURO"
	_, err := svc.Update(ctx, orgID, UpdateInput{Currency: &bad}, &actorID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "3-letter code")
}

func TestUpdateIntegrations(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	repo := newMockSettingsRepo()
	bus := &mockBus{}
	svc := NewService(repo, bus)

	svc.Get(ctx, orgID)

	s, err := svc.UpdateIntegrations(ctx, orgID, UpdateIntegrationsInput{
		Integrations: json.RawMessage(`{"atol_key":"abc123"}`),
	}, &actorID)

	require.NoError(t, err)
	assert.Contains(t, string(s.Integrations), "atol_key")
}

func TestSetLogo(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	fileID := uuid.New()
	ctx := context.Background()

	repo := newMockSettingsRepo()
	bus := &mockBus{}
	svc := NewService(repo, bus)

	svc.Get(ctx, orgID)

	s, err := svc.SetLogo(ctx, orgID, fileID, &actorID)
	require.NoError(t, err)
	assert.Equal(t, &fileID, s.LogoFileID)
}

func TestEnsureDefaults(t *testing.T) {
	orgID := uuid.New()
	ctx := context.Background()

	repo := newMockSettingsRepo()
	bus := &mockBus{}
	svc := NewService(repo, bus)

	err := svc.EnsureDefaults(ctx, orgID)
	require.NoError(t, err)

	s, err := svc.Get(ctx, orgID)
	require.NoError(t, err)
	assert.Equal(t, "RUB", s.Currency)

	err = svc.EnsureDefaults(ctx, orgID)
	require.NoError(t, err)
}
