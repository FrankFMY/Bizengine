package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bizengine/engine/pkg/errs"
	"github.com/bizengine/engine/pkg/types"
)

// --- mock repo ---

type mockRepo struct {
	users      map[string]*types.User
	usersById  map[uuid.UUID]*types.User
	organizations map[uuid.UUID]*types.Organization
	members    map[string]*types.Labor
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		users:      make(map[string]*types.User),
		usersById:  make(map[uuid.UUID]*types.User),
		organizations: make(map[uuid.UUID]*types.Organization),
		members:    make(map[string]*types.Labor),
	}
}

func (m *mockRepo) CreateUser(_ context.Context, u *types.User) error {
	if _, ok := m.users[u.Email]; ok {
		return errs.NewConflict("email exists")
	}
	cp := *u
	m.users[u.Email] = &cp
	m.usersById[u.ID] = &cp
	return nil
}

func (m *mockRepo) GetUserByEmail(_ context.Context, email string) (*types.User, error) {
	u, ok := m.users[email]
	if !ok {
		return nil, errs.NewNotFound("not found")
	}
	cp := *u
	return &cp, nil
}

func (m *mockRepo) GetUserByID(_ context.Context, id uuid.UUID) (*types.User, error) {
	u, ok := m.usersById[id]
	if !ok {
		return nil, errs.NewNotFound("not found")
	}
	cp := *u
	return &cp, nil
}

func (m *mockRepo) CreateOrganization(_ context.Context, org *types.Organization) error {
	cp := *org
	m.organizations[org.ID] = &cp
	return nil
}

func (m *mockRepo) GetOrganization(_ context.Context, id uuid.UUID) (*types.Organization, error) {
	org, ok := m.organizations[id]
	if !ok {
		return nil, errs.NewNotFound("not found")
	}
	cp := *org
	return &cp, nil
}

func (m *mockRepo) GetOrganizationBySlug(_ context.Context, slug string) (*types.Organization, error) {
	for _, org := range m.organizations {
		if org.Slug == slug {
			cp := *org
			return &cp, nil
		}
	}
	return nil, errs.NewNotFound("not found")
}

func (m *mockRepo) ListUserOrganizations(_ context.Context, userID uuid.UUID) ([]types.Organization, error) {
	var result []types.Organization
	for _, mem := range m.members {
		if mem.UserID == userID {
			if org, ok := m.organizations[mem.OrganizationID]; ok {
				result = append(result, *org)
			}
		}
	}
	return result, nil
}

func (m *mockRepo) UpdateOrganization(_ context.Context, org *types.Organization) error {
	m.organizations[org.ID] = org
	return nil
}

func (m *mockRepo) AddMember(_ context.Context, mem *types.Labor) error {
	key := mem.OrganizationID.String() + ":" + mem.UserID.String()
	cp := *mem
	m.members[key] = &cp
	return nil
}

func (m *mockRepo) GetMember(_ context.Context, orgID, userID uuid.UUID) (*types.Labor, error) {
	key := orgID.String() + ":" + userID.String()
	mem, ok := m.members[key]
	if !ok {
		return nil, errs.NewNotFound("not found")
	}
	cp := *mem
	return &cp, nil
}

func (m *mockRepo) ListMembers(_ context.Context, _ uuid.UUID) ([]types.Labor, error) {
	return nil, nil
}

func (m *mockRepo) UpdateMemberRole(_ context.Context, _, _ uuid.UUID, _ string) error {
	return nil
}

func (m *mockRepo) RemoveMember(_ context.Context, _, _ uuid.UUID) error {
	return nil
}

// --- mock session store ---

type mockStore struct {
	sessions map[string]*Session
	seances  map[string]*Seance
	sesSet   map[string][]string
}

func newMockStore() *mockStore {
	return &mockStore{
		sessions: make(map[string]*Session),
		seances:  make(map[string]*Seance),
		sesSet:   make(map[string][]string),
	}
}

func (m *mockStore) CreateSession(_ context.Context, s *Session, _ time.Duration) error {
	cp := *s
	m.sessions[s.ID] = &cp
	return nil
}

func (m *mockStore) GetSession(_ context.Context, id string) (*Session, error) {
	s, ok := m.sessions[id]
	if !ok {
		return nil, errs.NewUnauthorized("session expired")
	}
	cp := *s
	return &cp, nil
}

func (m *mockStore) UpdateSession(_ context.Context, s *Session, _ time.Duration) error {
	cp := *s
	m.sessions[s.ID] = &cp
	return nil
}

func (m *mockStore) DeleteSession(_ context.Context, id string) error {
	delete(m.sessions, id)
	return nil
}

func (m *mockStore) CreateSeance(_ context.Context, s *Seance, _ time.Duration) error {
	cp := *s
	m.seances[s.ID] = &cp
	m.sesSet[s.SessionID] = append(m.sesSet[s.SessionID], s.ID)
	return nil
}

func (m *mockStore) GetSeance(_ context.Context, id string) (*Seance, error) {
	s, ok := m.seances[id]
	if !ok {
		return nil, errs.NewUnauthorized("seance expired")
	}
	cp := *s
	return &cp, nil
}

func (m *mockStore) SlideSeance(_ context.Context, _ string, _ time.Duration) error {
	return nil
}

func (m *mockStore) DeleteSessionSeances(_ context.Context, sessionID string) error {
	for _, id := range m.sesSet[sessionID] {
		delete(m.seances, id)
	}
	delete(m.sesSet, sessionID)
	return nil
}

// --- tests ---

func TestHasPermission_Owner(t *testing.T) {
	assert.True(t, HasPermission("owner", nil, "entity.create"))
	assert.True(t, HasPermission("owner", nil, "finance.manage"))
	assert.True(t, HasPermission("owner", nil, "settings.manage"))
}

func TestHasPermission_Viewer(t *testing.T) {
	assert.True(t, HasPermission("viewer", nil, "entity.read"))
	assert.True(t, HasPermission("viewer", nil, "order.view"))
	assert.False(t, HasPermission("viewer", nil, "entity.create"))
	assert.False(t, HasPermission("viewer", nil, "finance.manage"))
}

func TestHasPermission_Operator(t *testing.T) {
	assert.True(t, HasPermission("operator", nil, "entity.read"))
	assert.True(t, HasPermission("operator", nil, "warehouse.receive"))
	assert.True(t, HasPermission("operator", nil, "warehouse.ship"))
	assert.False(t, HasPermission("operator", nil, "warehouse.adjust"))
	assert.False(t, HasPermission("operator", nil, "entity.create"))
}

func TestHasPermission_Manager(t *testing.T) {
	assert.True(t, HasPermission("manager", nil, "entity.create"))
	assert.True(t, HasPermission("manager", nil, "catalog.manage"))
	assert.True(t, HasPermission("manager", nil, "order.create"))
	assert.True(t, HasPermission("manager", nil, "finance.view"))
	assert.False(t, HasPermission("manager", nil, "finance.manage"))
	assert.False(t, HasPermission("manager", nil, "settings.manage"))
}

func TestHasPermission_CustomPerms(t *testing.T) {
	assert.True(t, HasPermission("viewer", []string{"finance.manage"}, "finance.manage"))
	assert.True(t, HasPermission("viewer", []string{"entity.create"}, "entity.create"))
}

func TestHasPermission_UnknownRole(t *testing.T) {
	assert.False(t, HasPermission("unknown", nil, "entity.read"))
}

func TestRegister_CreatesSessionAndSeance(t *testing.T) {
	repo := newMockRepo()
	store := newMockStore()
	svc := NewService(repo, store, 72*time.Hour, 10*time.Minute)

	result, err := svc.Register(context.Background(), RegisterInput{
		Email:    "test@example.com",
		Password: "password123",
		FullName: "Test User",
	})

	require.NoError(t, err)
	assert.Equal(t, "test@example.com", result.User.Email)
	assert.NotNil(t, result.Session)
	assert.NotNil(t, result.Seance)
	assert.NotEmpty(t, result.Session.ID)
	assert.Equal(t, result.Session.ID, result.Seance.SessionID)
}

func TestRegister_ShortPassword(t *testing.T) {
	repo := newMockRepo()
	store := newMockStore()
	svc := NewService(repo, store, 72*time.Hour, 10*time.Minute)

	_, err := svc.Register(context.Background(), RegisterInput{
		Email:    "test@example.com",
		Password: "short",
		FullName: "Test User",
	})
	assert.Error(t, err)
}

func TestLogin_Success(t *testing.T) {
	repo := newMockRepo()
	store := newMockStore()
	svc := NewService(repo, store, 72*time.Hour, 10*time.Minute)

	_, err := svc.Register(context.Background(), RegisterInput{
		Email:    "login@example.com",
		Password: "password123",
		FullName: "Test User",
	})
	require.NoError(t, err)

	result, err := svc.Login(context.Background(), LoginInput{
		Email:    "login@example.com",
		Password: "password123",
	})
	require.NoError(t, err)
	assert.NotNil(t, result.Session)
	assert.NotNil(t, result.Seance)
}

func TestLogin_WrongPassword(t *testing.T) {
	repo := newMockRepo()
	store := newMockStore()
	svc := NewService(repo, store, 72*time.Hour, 10*time.Minute)

	svc.Register(context.Background(), RegisterInput{
		Email: "login@example.com", Password: "password123", FullName: "Test",
	})

	_, err := svc.Login(context.Background(), LoginInput{
		Email: "login@example.com", Password: "wrong",
	})
	assert.Error(t, err)
}

func TestLogout_DeletesSessionAndSeances(t *testing.T) {
	repo := newMockRepo()
	store := newMockStore()
	svc := NewService(repo, store, 72*time.Hour, 10*time.Minute)

	result, _ := svc.Register(context.Background(), RegisterInput{
		Email: "logout@example.com", Password: "password123", FullName: "Test",
	})

	err := svc.Logout(context.Background(), result.Session.ID)
	require.NoError(t, err)

	_, err = store.GetSession(context.Background(), result.Session.ID)
	assert.Error(t, err)

	_, err = store.GetSeance(context.Background(), result.Seance.ID)
	assert.Error(t, err)
}

func TestUnlock_CreatesNewSeance(t *testing.T) {
	repo := newMockRepo()
	store := newMockStore()
	svc := NewService(repo, store, 72*time.Hour, 10*time.Minute)

	result, _ := svc.Register(context.Background(), RegisterInput{
		Email: "unlock@example.com", Password: "password123", FullName: "Test",
	})

	// Simulate seance expiry
	delete(store.seances, result.Seance.ID)

	seance, err := svc.Unlock(context.Background(), result.Session.ID, "password123")
	require.NoError(t, err)
	assert.NotEmpty(t, seance.ID)
	assert.Equal(t, result.Session.ID, seance.SessionID)
}

func TestUnlock_WrongPassword(t *testing.T) {
	repo := newMockRepo()
	store := newMockStore()
	svc := NewService(repo, store, 72*time.Hour, 10*time.Minute)

	result, _ := svc.Register(context.Background(), RegisterInput{
		Email: "unlock2@example.com", Password: "password123", FullName: "Test",
	})

	_, err := svc.Unlock(context.Background(), result.Session.ID, "wrong")
	assert.Error(t, err)
}

func TestSwitchOrganization(t *testing.T) {
	repo := newMockRepo()
	store := newMockStore()
	svc := NewService(repo, store, 72*time.Hour, 10*time.Minute)

	result, _ := svc.Register(context.Background(), RegisterInput{
		Email: "switch@example.com", Password: "password123", FullName: "Test",
	})

	org, err := svc.CreateOrganization(context.Background(), result.User.ID, "Test WS", "test-org")
	require.NoError(t, err)

	sess, err := svc.SwitchOrganization(context.Background(), result.Session.ID, org.ID)
	require.NoError(t, err)
	assert.Equal(t, org.ID, sess.OrganizationID)
	assert.Equal(t, "owner", sess.Role)
}

func TestSwitchOrganization_NotMember(t *testing.T) {
	repo := newMockRepo()
	store := newMockStore()
	svc := NewService(repo, store, 72*time.Hour, 10*time.Minute)

	result, _ := svc.Register(context.Background(), RegisterInput{
		Email: "switch2@example.com", Password: "password123", FullName: "Test",
	})

	_, err := svc.SwitchOrganization(context.Background(), result.Session.ID, uuid.New())
	assert.Error(t, err)
}
