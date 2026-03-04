package auth

import (
	"net/http"
	"strings"
)

// Default permissions for each role.
var rolePermissions = map[string]map[string]bool{
	"owner": {
		"entity.create": true, "entity.read": true, "entity.update": true, "entity.delete": true,
		"catalog.manage":    true,
		"warehouse.receive": true, "warehouse.ship": true, "warehouse.transfer": true, "warehouse.adjust": true,
		"order.create": true, "order.update": true, "order.cancel": true, "order.view": true,
		"hr.manage": true, "hr.view": true,
		"finance.manage": true, "finance.view": true,
		"logistics.manage": true, "logistics.view": true,
		"process.manage":  true,
		"settings.manage": true,
		"members.manage":  true,
	},
	"admin": {
		"entity.create": true, "entity.read": true, "entity.update": true, "entity.delete": true,
		"catalog.manage":    true,
		"warehouse.receive": true, "warehouse.ship": true, "warehouse.transfer": true, "warehouse.adjust": true,
		"order.create": true, "order.update": true, "order.cancel": true, "order.view": true,
		"hr.manage": true, "hr.view": true,
		"finance.manage": true, "finance.view": true,
		"logistics.manage": true, "logistics.view": true,
		"process.manage":  true,
		"settings.manage": true,
		"members.manage":  true,
	},
	"manager": {
		"entity.create": true, "entity.read": true, "entity.update": true,
		"catalog.manage":    true,
		"warehouse.receive": true, "warehouse.ship": true, "warehouse.transfer": true, "warehouse.adjust": true,
		"order.create": true, "order.update": true, "order.cancel": true, "order.view": true,
		"finance.view":     true,
		"logistics.manage": true, "logistics.view": true,
	},
	"operator": {
		"entity.read":      true,
		"warehouse.receive": true, "warehouse.ship": true,
		"order.view": true,
	},
	"viewer": {
		"entity.read": true,
		"order.view":  true,
	},
}

// HasPermission checks if a role (plus optional custom permissions) grants the required permission.
func HasPermission(role string, customPerms []string, required string) bool {
	// Check custom permissions first
	for _, p := range customPerms {
		if p == required {
			return true
		}
	}

	// Check role defaults
	perms, ok := rolePermissions[role]
	if !ok {
		return false
	}

	// Direct match
	if perms[required] {
		return true
	}

	// Wildcard: "entity.*" grants "entity.create"
	parts := strings.SplitN(required, ".", 2)
	if len(parts) == 2 {
		if perms[parts[0]+".*"] {
			return true
		}
	}

	return false
}

// RequirePermission returns an HTTP middleware that checks for a specific permission.
func RequirePermission(required string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, _ := r.Context().Value(ctxKeyRole).(string)
			if role == "" {
				http.Error(w, `{"code":"UNAUTHORIZED","message":"no role in context"}`, http.StatusUnauthorized)
				return
			}

			if !HasPermission(role, nil, required) {
				http.Error(w, `{"code":"FORBIDDEN","message":"insufficient permissions"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
