package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type contextKey string

const (
	ctxKeyUserID      contextKey = "user_id"
	ctxKeyWorkspaceID contextKey = "workspace_id"
	ctxKeyRole        contextKey = "role"
)

// Middleware extracts JWT from Authorization header and populates context.
func Middleware(authSvc *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				http.Error(w, `{"code":"UNAUTHORIZED","message":"missing authorization header"}`, http.StatusUnauthorized)
				return
			}

			tokenStr := strings.TrimPrefix(header, "Bearer ")
			if tokenStr == header {
				http.Error(w, `{"code":"UNAUTHORIZED","message":"invalid authorization format"}`, http.StatusUnauthorized)
				return
			}

			claims, err := authSvc.VerifyToken(tokenStr)
			if err != nil {
				http.Error(w, `{"code":"UNAUTHORIZED","message":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			userID, err := uuid.Parse(claims.Subject)
			if err != nil {
				http.Error(w, `{"code":"UNAUTHORIZED","message":"invalid user id in token"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), ctxKeyUserID, userID)
			ctx = context.WithValue(ctx, ctxKeyRole, claims.Role)

			if claims.WsID != "" {
				wsID, err := uuid.Parse(claims.WsID)
				if err == nil {
					ctx = context.WithValue(ctx, ctxKeyWorkspaceID, wsID)
				}
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromCtx extracts user_id from context.
func UserIDFromCtx(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxKeyUserID).(uuid.UUID)
	return id, ok
}

// WorkspaceIDFromCtx extracts workspace_id from context.
func WorkspaceIDFromCtx(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxKeyWorkspaceID).(uuid.UUID)
	return id, ok
}

// RoleFromCtx extracts role from context.
func RoleFromCtx(ctx context.Context) string {
	role, _ := ctx.Value(ctxKeyRole).(string)
	return role
}
