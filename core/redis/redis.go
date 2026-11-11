// Package redis holds the the API Redis client, a health check, and the
// server-side session store.
//
// Sessions live server-side so the API can end one (sign-out) and so they
// survive an API restart or deploy. The session cookie carries only an
// opaque id; the session itself is keyed by that id in Redis and expires by
// TTL.
package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// ErrNotFound is returned by Store.Get when the session does not exist,
// either because it was deleted or its TTL has expired.
var ErrNotFound = errors.New("redis: session not found")

// Client is the go-redis client. Callers can hold one without importing
// go-redis themselves.
type Client = goredis.Client

// Config configures the Redis client.
type Config struct {
	// Addr is the Redis server address, e.g. "127.0.0.1:6379".
	Addr string
	// Password is the ACL password. It is empty when the server requires no
	// authentication.
	Password string
}

// NewClient builds a go-redis client for cfg and returns it once the server
// answers a Ping. It closes the client if the Ping fails.
func NewClient(ctx context.Context, cfg Config) (*goredis.Client, error) {
	client := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
	})
	if err := Ping(ctx, client); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

// Ping is the health check: it succeeds only when the server answers a
// PING command.
func Ping(ctx context.Context, client *goredis.Client) error {
	if err := client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping: %w", err)
	}
	return nil
}

// Store reads and writes sessions in Redis.
type Store struct {
	client *goredis.Client
	// prefix is prepended to every session key so independent stores (and
	// tests) can share one server without colliding.
	prefix string
}

// NewStore returns a Store that uses client. prefix is prepended to every
// session key; it may be empty.
func NewStore(client *goredis.Client, prefix string) *Store {
	return &Store{client: client, prefix: prefix}
}

// idBytes is the number of random bytes in an opaque session id.
const idBytes = 32

// newID returns an opaque, unpredictable session id.
func newID() (string, error) {
	buf := make([]byte, idBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func (s *Store) key(id string) string {
	return s.prefix + id
}

// Create stores a new session holding data and returns its opaque id. The
// session expires after ttl.
func (s *Store) Create(ctx context.Context, ttl time.Duration, data map[string]string) (string, error) {
	// Redis treats a zero TTL as "never expire", so refuse it rather than
	// mint a session that outlives every sign-out policy.
	if ttl <= 0 {
		return "", fmt.Errorf("create session: ttl must be positive, got %v", ttl)
	}
	id, err := newID()
	if err != nil {
		return "", err
	}
	payload, err := encode(data)
	if err != nil {
		return "", err
	}
	if err := s.client.Set(ctx, s.key(id), payload, ttl).Err(); err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	return id, nil
}

// Get returns the data of the session with the given id, or ErrNotFound if
// the session does not exist or has expired.
func (s *Store) Get(ctx context.Context, id string) (map[string]string, error) {
	payload, err := s.client.Get(ctx, s.key(id)).Result()
	if errors.Is(err, goredis.Nil) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	return decode(payload)
}

// Take returns the data of the record with the given id and deletes it in
// the same command (GETDEL), so of two concurrent callers presenting the
// same id at most one gets the data. It returns ErrNotFound if the record
// does not exist or has expired. It is for one-shot records such as an
// OAuth sign-in attempt.
func (s *Store) Take(ctx context.Context, id string) (map[string]string, error) {
	payload, err := s.client.GetDel(ctx, s.key(id)).Result()
	if errors.Is(err, goredis.Nil) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("take record: %w", err)
	}
	return decode(payload)
}

// Delete removes the session with the given id. It is idempotent: deleting a
// session that does not exist is not an error.
func (s *Store) Delete(ctx context.Context, id string) error {
	if err := s.client.Del(ctx, s.key(id)).Err(); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// encode serialises data as JSON. A nil map encodes as a valid (empty) JSON
// object.
func encode(data map[string]string) (string, error) {
	if data == nil {
		data = map[string]string{}
	}
	b, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("encode session: %w", err)
	}
	return string(b), nil
}

func decode(payload string) (map[string]string, error) {
	data := map[string]string{}
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		return nil, fmt.Errorf("decode session: %w", err)
	}
	return data, nil
}
