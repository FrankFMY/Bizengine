package rest

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bizengine/engine/internal/core/auth"
	"github.com/bizengine/engine/pkg/errs"
	"github.com/bizengine/engine/pkg/types"
)

type PassTempHandler struct {
	pool         *pgxpool.Pool
	authSvc      *auth.Service
	cookieSecure bool
}

func NewPassTempHandler(pool *pgxpool.Pool, authSvc *auth.Service, cookieSecure bool) *PassTempHandler {
	return &PassTempHandler{pool: pool, authSvc: authSvc, cookieSecure: cookieSecure}
}

type passTempInput struct {
	Data struct {
		Country string `json:"country"`
		Phone   string `json:"phone"`
		Secret  string `json:"secret"`
		Name    string `json:"name"`
		INN     string `json:"inn"`
	} `json:"data"`
}

var reNonDigit = regexp.MustCompile(`\D`)

func normalizePhone(raw string) (unformat, format string) {
	digits := reNonDigit.ReplaceAllString(raw, "")
	if len(digits) > 0 && digits[0] == '8' {
		digits = "7" + digits[1:]
	}
	unformat = digits
	if len(digits) == 11 && digits[0] == '7' {
		format = digits[0:1] + " (" + digits[1:4] + ") " + digits[4:7] + "-" + digits[7:9] + "-" + digits[9:11]
	} else {
		format = digits
	}
	return
}

func (h *PassTempHandler) Handle(w http.ResponseWriter, r *http.Request) {
	var input passTempInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	d := input.Data
	if strings.TrimSpace(d.Phone) == "" {
		respondError(w, errs.NewBadRequest("phone is required"))
		return
	}
	if strings.TrimSpace(d.Secret) == "" {
		respondError(w, errs.NewBadRequest("secret is required"))
		return
	}
	if strings.TrimSpace(d.Name) == "" {
		respondError(w, errs.NewBadRequest("name is required"))
		return
	}
	if strings.TrimSpace(d.INN) == "" {
		respondError(w, errs.NewBadRequest("inn is required"))
		return
	}

	country := d.Country
	if country == "" {
		country = "RU"
	}

	unformat, format := normalizePhone(d.Phone)

	hash, err := auth.HashSecret(d.Secret)
	if err != nil {
		respondError(w, errs.NewInternal("failed to hash secret"))
		return
	}

	var userID uuid.UUID
	var orgID uuid.UUID
	var phoneID uuid.UUID

	err = pgxTransaction(r.Context(), h.pool, func(tx pgx.Tx) error {
		// Find or create user by phone
		var existingUserID *uuid.UUID
		err := tx.QueryRow(r.Context(),
			`SELECT u.id FROM users u JOIN phones p ON p.user_id = u.id WHERE p.unformat = $1 LIMIT 1`,
			unformat,
		).Scan(&existingUserID)

		now := time.Now()

		if err == pgx.ErrNoRows || existingUserID == nil {
			userID = uuid.New()
			name := d.Name
			_, err = tx.Exec(r.Context(),
				`INSERT INTO users (id, email, password_hash, full_name, phone, is_active, name, secret, created_at, updated_at)
				 VALUES ($1, $2, $3, $4, $5, true, $6, $7, $8, $9)`,
				userID, "", "", "", nil, &name, hash, now, now,
			)
			if err != nil {
				return err
			}

			phoneID = uuid.New()
			_, err = tx.Exec(r.Context(),
				`INSERT INTO phones (id, user_id, unformat, format, country, ver, upd, iat)
				 VALUES ($1, $2, $3, $4, $5, 1, $6, $7)`,
				phoneID, userID, unformat, format, country, now, now,
			)
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			userID = *existingUserID
			_, err = tx.Exec(r.Context(),
				`UPDATE users SET secret = $2, name = $3, updated_at = $4 WHERE id = $1`,
				userID, hash, d.Name, now,
			)
			if err != nil {
				return err
			}
			err = tx.QueryRow(r.Context(),
				`SELECT id FROM phones WHERE user_id = $1 AND unformat = $2 LIMIT 1`,
				userID, unformat,
			).Scan(&phoneID)
			if err != nil {
				return err
			}
		}

		// Find or create organization by INN
		inn := d.INN
		err = tx.QueryRow(r.Context(),
			`SELECT id FROM organizations WHERE inn = $1 LIMIT 1`, inn,
		).Scan(&orgID)

		if err == pgx.ErrNoRows {
			orgID = uuid.New()
			slug := "org-" + orgID.String()[:8]
			_, err = tx.Exec(r.Context(),
				`INSERT INTO organizations (id, name, slug, owner_id, plan, settings, inn, created_at, updated_at)
				 VALUES ($1, $2, $3, $4, 'free', '{}', $5, $6, $7)`,
				orgID, d.Name, slug, userID, inn, now, now,
			)
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		// Find or create labor
		var laborID uuid.UUID
		err = tx.QueryRow(r.Context(),
			`SELECT id FROM labors WHERE organization_id = $1 AND user_id = $2`,
			orgID, userID,
		).Scan(&laborID)

		if err == pgx.ErrNoRows {
			laborID = uuid.New()
			_, err = tx.Exec(r.Context(),
				`INSERT INTO labors (id, organization_id, user_id, role, permissions, admin, ver, upd, iat, joined_at)
				 VALUES ($1, $2, $3, 'owner', '[]', true, 1, $4, $5, $6)`,
				laborID, orgID, userID, now, now, now,
			)
			if err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		respondError(w, errs.Wrap(err, errs.CodeInternal, "transaction failed"))
		return
	}

	user := &types.User{
		ID:       userID,
		IsActive: true,
	}
	sess, seance, err := h.authSvc.CreateSessionAndSeance(r.Context(), user, phoneID, orgID, "owner")
	if err != nil {
		respondError(w, errs.Wrap(err, errs.CodeInternal, "failed to create session"))
		return
	}

	auth.SetAuthCookies(w, sess, seance, h.authSvc.SessionTTL(), h.authSvc.SeanceTTL(), h.cookieSecure)
	respondOK(w, http.StatusOK, nil)
}

func pgxTransaction(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
