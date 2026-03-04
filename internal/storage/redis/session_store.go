package redis

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/bizengine/engine/internal/core/auth"
	"github.com/bizengine/engine/pkg/errs"
)

const (
	sessionPrefix = "teco:session:"
	seancePrefix  = "teco:seance:"
	seanceSetKey  = "teco:session_seances:"
)

// SessionStore implements auth.SessionStore using Redis.
type SessionStore struct {
	client *redis.Client
}

// NewSessionStore creates a new Redis-backed session store.
func NewSessionStore(client *redis.Client) *SessionStore {
	return &SessionStore{client: client}
}

// CreateSession stores a session in Redis with the given TTL.
func (s *SessionStore) CreateSession(ctx context.Context, sess *auth.Session, ttl time.Duration) error {
	data, err := json.Marshal(sess)
	if err != nil {
		return errs.Wrap(err, errs.CodeInternal, "failed to marshal session")
	}
	return s.client.Set(ctx, sessionPrefix+sess.ID, data, ttl).Err()
}

// GetSession retrieves a session by ID.
func (s *SessionStore) GetSession(ctx context.Context, id string) (*auth.Session, error) {
	data, err := s.client.Get(ctx, sessionPrefix+id).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, errs.NewUnauthorized("session expired")
		}
		return nil, err
	}
	var sess auth.Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to unmarshal session")
	}
	return &sess, nil
}

// UpdateSession overwrites a session in Redis, preserving its TTL.
func (s *SessionStore) UpdateSession(ctx context.Context, sess *auth.Session, ttl time.Duration) error {
	data, err := json.Marshal(sess)
	if err != nil {
		return errs.Wrap(err, errs.CodeInternal, "failed to marshal session")
	}
	key := sessionPrefix + sess.ID

	remaining, err := s.client.TTL(ctx, key).Result()
	if err != nil || remaining <= 0 {
		remaining = ttl
	}

	return s.client.Set(ctx, key, data, remaining).Err()
}

// DeleteSession removes a session from Redis.
func (s *SessionStore) DeleteSession(ctx context.Context, id string) error {
	return s.client.Del(ctx, sessionPrefix+id).Err()
}

// CreateSeance stores a seance in Redis and tracks it under its session.
func (s *SessionStore) CreateSeance(ctx context.Context, seance *auth.Seance, ttl time.Duration) error {
	data, err := json.Marshal(seance)
	if err != nil {
		return errs.Wrap(err, errs.CodeInternal, "failed to marshal seance")
	}

	pipe := s.client.Pipeline()
	pipe.Set(ctx, seancePrefix+seance.ID, data, ttl)
	pipe.SAdd(ctx, seanceSetKey+seance.SessionID, seance.ID)
	_, err = pipe.Exec(ctx)
	return err
}

// GetSeance retrieves a seance by ID.
func (s *SessionStore) GetSeance(ctx context.Context, id string) (*auth.Seance, error) {
	data, err := s.client.Get(ctx, seancePrefix+id).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, errs.NewUnauthorized("seance expired")
		}
		return nil, err
	}
	var seance auth.Seance
	if err := json.Unmarshal(data, &seance); err != nil {
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to unmarshal seance")
	}
	return &seance, nil
}

// SlideSeance extends a seance's TTL (sliding window).
func (s *SessionStore) SlideSeance(ctx context.Context, id string, ttl time.Duration) error {
	return s.client.Expire(ctx, seancePrefix+id, ttl).Err()
}

// DeleteSessionSeances removes all seances belonging to a session.
func (s *SessionStore) DeleteSessionSeances(ctx context.Context, sessionID string) error {
	setKey := seanceSetKey + sessionID

	ids, err := s.client.SMembers(ctx, setKey).Result()
	if err != nil {
		return err
	}

	if len(ids) == 0 {
		return nil
	}

	keys := make([]string, 0, len(ids)+1)
	for _, id := range ids {
		keys = append(keys, seancePrefix+id)
	}
	keys = append(keys, setKey)

	return s.client.Del(ctx, keys...).Err()
}
