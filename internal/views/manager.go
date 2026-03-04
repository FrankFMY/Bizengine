package views

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/bizengine/engine/pkg/errs"
)

// Manager coordinates view subscriptions, data caching, and invalidation.
type Manager struct {
	registry  *Registry
	pool      *pgxpool.Pool
	publisher ViewPublisher

	mu        sync.RWMutex
	subs      map[string][]*Subscription        // seance_id -> subscriptions
	dataStore map[string]*WorkspaceDataStore     // workspace_id string -> store
}

// Subscription tracks a single client's view subscription.
type Subscription struct {
	SeanceID    string
	ViewKey     string
	Params      map[string]any
	ParamsHash  string
	WorkspaceID uuid.UUID
	UserID      uuid.UUID
	LastRefs    []DataRef
	Version     int64
}

// WorkspaceDataStore holds the normalized data cache for a single workspace.
type WorkspaceDataStore struct {
	mu     sync.RWMutex
	tables map[string]map[string]*DataRow // table -> id -> row
}

// DataRow is a cached row with reference counting.
type DataRow struct {
	Fields   map[string]any
	Version  int64
	RefCount int
}

// NewManager creates a Manager with the given dependencies.
func NewManager(registry *Registry, pool *pgxpool.Pool, publisher ViewPublisher) *Manager {
	return &Manager{
		registry:  registry,
		pool:      pool,
		publisher: publisher,
		subs:      make(map[string][]*Subscription),
		dataStore: make(map[string]*WorkspaceDataStore),
	}
}

// SubscribeResult is returned to the HTTP handler after subscribing.
type SubscribeResult struct {
	ParamsHash string `json:"params_hash"`
	Version    int64  `json:"version"`
}

// Subscribe creates a new view subscription, runs the factory, and sends a snapshot.
func (m *Manager) Subscribe(ctx context.Context, seanceID string, wsID, userID uuid.UUID, viewKey string, params map[string]any) (*SubscribeResult, error) {
	def, ok := m.registry.Get(viewKey)
	if !ok {
		return nil, errs.NewNotFound("view not found: " + viewKey)
	}

	if err := validateParams(def.ParamSchema, params); err != nil {
		return nil, err
	}

	paramsHash := computeParamsHash(viewKey, params)

	// Check for duplicate subscription.
	m.mu.RLock()
	for _, sub := range m.subs[seanceID] {
		if sub.ParamsHash == paramsHash {
			m.mu.RUnlock()
			return &SubscribeResult{ParamsHash: paramsHash, Version: sub.Version}, nil
		}
	}
	m.mu.RUnlock()

	result, err := def.Factory(ctx, m.pool, wsID, params)
	if err != nil {
		return nil, errs.Wrap(err, errs.CodeInternal, "view factory failed")
	}

	sub := &Subscription{
		SeanceID:    seanceID,
		ViewKey:     viewKey,
		Params:      params,
		ParamsHash:  paramsHash,
		WorkspaceID: wsID,
		UserID:      userID,
		LastRefs:    result.Refs,
		Version:     result.Version,
	}

	m.mu.Lock()
	m.subs[seanceID] = append(m.subs[seanceID], sub)
	store := m.getOrCreateStore(wsID.String())
	m.mu.Unlock()

	// Add data to workspace store, incrementing ref counts.
	store.AddFromResult(result)

	// Send snapshot via Centrifugo.
	m.publisher.SendSnapshot(ctx, seanceID, ViewSnapshotMsg{
		View:       viewKey,
		ParamsHash: paramsHash,
		Version:    result.Version,
		Refs:       result.Refs,
		Tables:     result.Tables,
	})

	return &SubscribeResult{ParamsHash: paramsHash, Version: result.Version}, nil
}

// Unsubscribe removes a specific view subscription by params hash.
func (m *Manager) Unsubscribe(seanceID, paramsHash string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	subs := m.subs[seanceID]
	for i, sub := range subs {
		if sub.ParamsHash == paramsHash {
			store := m.dataStore[sub.WorkspaceID.String()]
			if store != nil {
				store.RemoveRefs(sub.LastRefs)
			}
			m.subs[seanceID] = append(subs[:i], subs[i+1:]...)
			return
		}
	}
}

// UnsubscribeAll removes all subscriptions for a seance (e.g., on logout).
func (m *Manager) UnsubscribeAll(seanceID string) {
	m.mu.Lock()
	subs := m.subs[seanceID]
	delete(m.subs, seanceID)
	m.mu.Unlock()

	for _, sub := range subs {
		m.mu.RLock()
		store := m.dataStore[sub.WorkspaceID.String()]
		m.mu.RUnlock()
		if store != nil {
			store.RemoveRefs(sub.LastRefs)
		}
	}
}

// ActiveSub describes one active subscription for the /views/active endpoint.
type ActiveSub struct {
	View       string `json:"view"`
	ParamsHash string `json:"params_hash"`
	Version    int64  `json:"version"`
}

// ActiveSubscriptions returns all active subscriptions for a seance.
func (m *Manager) ActiveSubscriptions(seanceID string) []ActiveSub {
	m.mu.RLock()
	defer m.mu.RUnlock()

	subs := m.subs[seanceID]
	out := make([]ActiveSub, len(subs))
	for i, s := range subs {
		out[i] = ActiveSub{View: s.ViewKey, ParamsHash: s.ParamsHash, Version: s.Version}
	}
	return out
}

// SyncRequest describes a client's cached state for reconnection.
type SyncRequest struct {
	Views  []SyncView                    `json:"views"`
	Tables map[string]map[string]int64   `json:"tables"` // table -> id -> version
}

// SyncView describes one view the client thinks it's subscribed to.
type SyncView struct {
	View       string `json:"view"`
	ParamsHash string `json:"params_hash"`
	Version    int64  `json:"version"`
}

// Sync handles reconnection: re-sends diffs or full snapshots as needed.
func (m *Manager) Sync(ctx context.Context, seanceID string, wsID uuid.UUID, req SyncRequest) error {
	m.mu.RLock()
	subs := m.subs[seanceID]
	m.mu.RUnlock()

	subMap := make(map[string]*Subscription)
	for _, s := range subs {
		subMap[s.ParamsHash] = s
	}

	for _, sv := range req.Views {
		sub, exists := subMap[sv.ParamsHash]
		if !exists {
			continue
		}

		versionDiff := sub.Version - sv.Version
		if versionDiff > 10 || versionDiff < 0 {
			// Too far behind or ahead: send full snapshot.
			def, ok := m.registry.Get(sub.ViewKey)
			if !ok {
				continue
			}
			result, err := def.Factory(ctx, m.pool, wsID, sub.Params)
			if err != nil {
				log.Error().Err(err).Str("view", sub.ViewKey).Msg("sync: factory failed")
				continue
			}

			m.publisher.SendSnapshot(ctx, seanceID, ViewSnapshotMsg{
				View:       sub.ViewKey,
				ParamsHash: sub.ParamsHash,
				Version:    result.Version,
				Refs:       result.Refs,
				Tables:     result.Tables,
			})

			m.mu.Lock()
			sub.LastRefs = result.Refs
			sub.Version = result.Version
			m.mu.Unlock()
		}
		// If version diff <= 10, the normal invalidation flow will catch up.
	}

	return nil
}

// Invalidate processes a ChangeEvent: computes table_diff and view_diff as needed.
func (m *Manager) Invalidate(ctx context.Context, change ChangeEvent) {
	wsKey := change.WorkspaceID.String()

	// 1. Table diff: check if the changed row exists in the workspace data store.
	m.mu.RLock()
	store := m.dataStore[wsKey]
	m.mu.RUnlock()

	if store != nil {
		oldFields := store.GetRow(change.Table, change.RowID)
		if oldFields != nil {
			// Row is cached — we'd normally re-read from DB here.
			// For MVP, we mark version bump and let the next view factory refresh catch it.
			// The actual re-read from DB would require table-specific SQL which belongs in Phase C.
			store.BumpVersion(change.Table, change.RowID)
		}
	}

	// 2. View diff: find affected subscriptions and re-run their factories.
	affected := m.registry.GetByTable(change.Table)
	if len(affected) == 0 {
		return
	}

	affectedKeys := make(map[string]bool)
	for _, def := range affected {
		affectedKeys[def.Key] = true
	}

	m.mu.RLock()
	var toProcess []*Subscription
	for _, subs := range m.subs {
		for _, sub := range subs {
			if sub.WorkspaceID == change.WorkspaceID && affectedKeys[sub.ViewKey] {
				toProcess = append(toProcess, sub)
			}
		}
	}
	m.mu.RUnlock()

	for _, sub := range toProcess {
		def, ok := m.registry.Get(sub.ViewKey)
		if !ok {
			continue
		}

		result, err := def.Factory(ctx, m.pool, sub.WorkspaceID, sub.Params)
		if err != nil {
			log.Error().Err(err).Str("view", sub.ViewKey).Msg("invalidate: factory failed")
			continue
		}

		refsPatch := ComputeRefsDiff(sub.LastRefs, result.Refs)
		if len(refsPatch) > 0 {
			// Collect only new records (refs added that weren't in LastRefs).
			newTables := collectNewTables(sub.LastRefs, result)

			m.publisher.SendViewDiff(ctx, sub.SeanceID, ViewDiffMsg{
				View:       sub.ViewKey,
				ParamsHash: sub.ParamsHash,
				Version:    result.Version,
				RefsPatch:  refsPatch,
				Tables:     newTables,
			})
		}

		m.mu.Lock()
		if store := m.dataStore[wsKey]; store != nil {
			store.RemoveRefs(sub.LastRefs)
			store.AddFromResult(result)
		}
		sub.LastRefs = result.Refs
		sub.Version = result.Version
		m.mu.Unlock()
	}
}

func (m *Manager) getOrCreateStore(wsKey string) *WorkspaceDataStore {
	store, ok := m.dataStore[wsKey]
	if !ok {
		store = &WorkspaceDataStore{
			tables: make(map[string]map[string]*DataRow),
		}
		m.dataStore[wsKey] = store
	}
	return store
}

// AddFromResult adds/increments ref counts for all records in a ViewResult.
func (s *WorkspaceDataStore) AddFromResult(result *ViewResult) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, ref := range result.Refs {
		tbl, ok := s.tables[ref.Table]
		if !ok {
			tbl = make(map[string]*DataRow)
			s.tables[ref.Table] = tbl
		}

		row, ok := tbl[ref.ID]
		if !ok {
			fields := extractFields(result.Tables, ref)
			row = &DataRow{Fields: fields, Version: result.Version}
			tbl[ref.ID] = row
		}
		row.RefCount++
	}
}

// RemoveRefs decrements ref counts and garbage-collects rows with RefCount == 0.
func (s *WorkspaceDataStore) RemoveRefs(refs []DataRef) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, ref := range refs {
		tbl, ok := s.tables[ref.Table]
		if !ok {
			continue
		}
		row, ok := tbl[ref.ID]
		if !ok {
			continue
		}
		row.RefCount--
		if row.RefCount <= 0 {
			delete(tbl, ref.ID)
		}
	}
}

// GetRow returns a copy of the cached fields for a row, or nil if not cached.
func (s *WorkspaceDataStore) GetRow(table, id string) map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tbl, ok := s.tables[table]
	if !ok {
		return nil
	}
	row, ok := tbl[id]
	if !ok {
		return nil
	}
	cp := make(map[string]any, len(row.Fields))
	for k, v := range row.Fields {
		cp[k] = v
	}
	return cp
}

// BumpVersion increments the version of a cached row.
func (s *WorkspaceDataStore) BumpVersion(table, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if tbl, ok := s.tables[table]; ok {
		if row, ok := tbl[id]; ok {
			row.Version++
		}
	}
}

func extractFields(tables map[string]map[string]any, ref DataRef) map[string]any {
	tbl, ok := tables[ref.Table]
	if !ok {
		return nil
	}
	row, ok := tbl[ref.ID]
	if !ok {
		return nil
	}
	m, ok := row.(map[string]any)
	if !ok {
		return nil
	}
	cp := make(map[string]any, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

func collectNewTables(oldRefs []DataRef, result *ViewResult) map[string]map[string]any {
	oldSet := make(map[string]struct{})
	for _, ref := range oldRefs {
		oldSet[ref.Table+":"+ref.ID] = struct{}{}
	}

	newTables := make(map[string]map[string]any)
	for _, ref := range result.Refs {
		key := ref.Table + ":" + ref.ID
		if _, existed := oldSet[key]; existed {
			continue
		}
		if result.Tables[ref.Table] == nil {
			continue
		}
		row, ok := result.Tables[ref.Table][ref.ID]
		if !ok {
			continue
		}
		if newTables[ref.Table] == nil {
			newTables[ref.Table] = make(map[string]any)
		}
		newTables[ref.Table][ref.ID] = row
	}

	if len(newTables) == 0 {
		return nil
	}
	return newTables
}

func validateParams(schema map[string]string, params map[string]any) error {
	for name, typ := range schema {
		val, ok := params[name]
		if !ok {
			continue // optional params
		}
		switch typ {
		case "uuid":
			s, ok := val.(string)
			if !ok {
				return errs.NewBadRequest("param " + name + " must be a string (uuid)")
			}
			if _, err := uuid.Parse(s); err != nil {
				return errs.NewBadRequest("param " + name + " is not a valid uuid")
			}
		case "int":
			switch val.(type) {
			case float64, int, int64:
			default:
				return errs.NewBadRequest("param " + name + " must be a number")
			}
		case "string":
			if _, ok := val.(string); !ok {
				return errs.NewBadRequest("param " + name + " must be a string")
			}
		}
	}
	return nil
}

func computeParamsHash(viewKey string, params map[string]any) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := sha256.New()
	h.Write([]byte(viewKey))
	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte(fmt.Sprintf("%v", params[k])))
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}
