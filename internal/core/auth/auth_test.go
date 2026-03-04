package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bizengine/engine/pkg/types"
)

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

func TestVerifyToken_RoundTrip(t *testing.T) {
	svc := NewService(nil, "test-secret-32-chars-minimum!!!!!", 15*time.Minute, 720*time.Hour)

	user := &types.User{
		ID:       uuid.New(),
		Email:    "test@example.com",
		FullName: "Test User",
	}
	wsID := uuid.New()

	token, err := svc.issueAccessToken(user, &wsID, "admin")
	require.NoError(t, err)
	require.NotEmpty(t, token)

	claims, err := svc.VerifyToken(token)
	require.NoError(t, err)
	assert.Equal(t, user.ID.String(), claims.Subject)
	assert.Equal(t, user.Email, claims.Email)
	assert.Equal(t, "admin", claims.Role)
	assert.Equal(t, wsID.String(), claims.WsID)
}

func TestVerifyToken_Invalid(t *testing.T) {
	svc := NewService(nil, "test-secret-32-chars-minimum!!!!!", 15*time.Minute, 720*time.Hour)

	_, err := svc.VerifyToken("invalid-token")
	assert.Error(t, err)
}

func TestVerifyToken_WrongSecret(t *testing.T) {
	svc1 := NewService(nil, "secret-one-32-chars-minimum!!!!!", 15*time.Minute, 720*time.Hour)
	svc2 := NewService(nil, "secret-two-32-chars-minimum!!!!!", 15*time.Minute, 720*time.Hour)

	user := &types.User{ID: uuid.New(), Email: "test@example.com"}
	token, _ := svc1.issueAccessToken(user, nil, "admin")

	_, err := svc2.VerifyToken(token)
	assert.Error(t, err)
}
