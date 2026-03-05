package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type contextKey string

const (
	ctxKeyUserID         contextKey = "user_id"
	ctxKeyOrganizationID contextKey = "organization_id"
	ctxKeyRole           contextKey = "role"
	ctxKeyPermissions    contextKey = "permissions"
	ctxKeySessionID      contextKey = "session_id"
	ctxKeySeanceID       contextKey = "seance_id"
)

// Exported context keys for testing.
var (
	ExportedCtxKeyUserID      = ctxKeyUserID
	ExportedCtxKeyOrganizationID = ctxKeyOrganizationID
	ExportedCtxKeyRole        = ctxKeyRole
)

// ExportedCtxKeySeanceID returns the seance context key for testing.
func ExportedCtxKeySeanceID() contextKey { return ctxKeySeanceID }

const (
	cookieSession = "teco_session"
	cookieSeance  = "teco_seance"
)

// Middleware reads session/seance cookies and populates context.
func Middleware(svc *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sessionCookie, err := r.Cookie(cookieSession)
			if err != nil || sessionCookie.Value == "" {
				writeAuthPlain(w, "SESSION")
				return
			}

			store := svc.Store()
			ctx := r.Context()

			sess, err := store.GetSession(ctx, sessionCookie.Value)
			if err != nil {
				writeAuthPlain(w, "SESSION")
				return
			}

			seanceCookie, err := r.Cookie(cookieSeance)
			if err != nil || seanceCookie.Value == "" {
				writeAuthPlain(w, "SEANCE")
				return
			}

			seance, err := store.GetSeance(ctx, seanceCookie.Value)
			if err != nil {
				writeAuthPlain(w, "SEANCE")
				return
			}

			if seance.SessionID != sessionCookie.Value {
				writeAuthPlain(w, "SEANCE")
				return
			}

			store.SlideSeance(ctx, seanceCookie.Value, svc.SeanceTTL())

			ctx = context.WithValue(ctx, ctxKeyUserID, sess.UserID)
			ctx = context.WithValue(ctx, ctxKeyRole, sess.Role)
			ctx = context.WithValue(ctx, ctxKeyPermissions, sess.Permissions)
			ctx = context.WithValue(ctx, ctxKeySessionID, sess.ID)
			ctx = context.WithValue(ctx, ctxKeySeanceID, seanceCookie.Value)

			if sess.OrganizationID != uuid.Nil {
				ctx = context.WithValue(ctx, ctxKeyOrganizationID, sess.OrganizationID)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SessionOnlyMiddleware validates only the session cookie (for /unlock where seance is expired).
func SessionOnlyMiddleware(svc *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sessionCookie, err := r.Cookie(cookieSession)
			if err != nil || sessionCookie.Value == "" {
				writeAuthPlain(w, "SESSION")
				return
			}

			sess, err := svc.Store().GetSession(r.Context(), sessionCookie.Value)
			if err != nil {
				writeAuthPlain(w, "SESSION")
				return
			}

			ctx := context.WithValue(r.Context(), ctxKeyUserID, sess.UserID)
			ctx = context.WithValue(ctx, ctxKeySessionID, sess.ID)
			ctx = context.WithValue(ctx, ctxKeyRole, sess.Role)

			if sess.OrganizationID != uuid.Nil {
				ctx = context.WithValue(ctx, ctxKeyOrganizationID, sess.OrganizationID)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SetAuthCookies sets teco_session and teco_seance cookies.
func SetAuthCookies(w http.ResponseWriter, sess *Session, seance *Seance, sessionTTL, seanceTTL time.Duration, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieSession,
		Value:    sess.ID,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     cookieSeance,
		Value:    seance.ID,
		Path:     "/",
		MaxAge:   int(seanceTTL.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearAuthCookies removes auth cookies.
func ClearAuthCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieSession,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     cookieSeance,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// UserIDFromCtx extracts user_id from context.
func UserIDFromCtx(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxKeyUserID).(uuid.UUID)
	return id, ok
}

// OrganizationIDFromCtx extracts organization_id from context.
func OrganizationIDFromCtx(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxKeyOrganizationID).(uuid.UUID)
	return id, ok
}

// RoleFromCtx extracts role from context.
func RoleFromCtx(ctx context.Context) string {
	role, _ := ctx.Value(ctxKeyRole).(string)
	return role
}

// PermissionsFromCtx extracts custom permissions from context.
func PermissionsFromCtx(ctx context.Context) []string {
	perms, _ := ctx.Value(ctxKeyPermissions).([]string)
	return perms
}

// SessionIDFromCtx extracts session ID from context.
func SessionIDFromCtx(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(ctxKeySessionID).(string)
	return id, ok
}

// SeanceIDFromCtx extracts seance ID from context.
func SeanceIDFromCtx(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(ctxKeySeanceID).(string)
	return id, ok
}

func writeAuthPlain(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`"` + body + `"`))
}
