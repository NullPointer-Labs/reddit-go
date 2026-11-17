package store

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

const stream = "comments.events"

var (
	errBody   = &Error{400, "body required"}
	errCreate = &Error{400, "post_id and author required"}
	errDelta  = &Error{400, "delta must be 1 or -1"}
)

type CreateInput struct {
	PostID *int64 `json:"post_id"`
	Author string `json:"author"`
	Body   string `json:"body"`
}

func (in CreateInput) validate() error {
	if strings.TrimSpace(in.Body) == "" {
		return errBody
	}
	if in.PostID == nil || in.Author == "" {
		return errCreate
	}
	return nil
}

func (s *Store) Create(ctx context.Context, in CreateInput, forcedID int64) (Comment, error) {
	if err := in.validate(); err != nil {
		return Comment{}, err
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	body := strings.TrimSpace(in.Body)
	var row pgx.Row
	if forcedID != 0 {
		row = s.db.QueryRow(ctx, "INSERT INTO comments (id, post_id, author, body, created_at, updated_at) "+
			"VALUES ($1, $2, $3, $4, $5, $5) RETURNING "+columns, forcedID, *in.PostID, in.Author, body, now)
	} else {
		row = s.db.QueryRow(ctx, "INSERT INTO comments (post_id, author, body, created_at, updated_at) "+
			"VALUES ($1, $2, $3, $4, $4) RETURNING "+columns, *in.PostID, in.Author, body, now)
	}
	c, err := scanComment(row)
	if err != nil {
		return c, err
	}
	return c, s.publish(ctx, "comment_created", c)
}

func (s *Store) Update(ctx context.Context, id int64, body string) (Comment, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return Comment{}, errBody
	}
	c, err := scanComment(s.db.QueryRow(ctx, "UPDATE comments "+
		"SET body = $2, edited = true, version = version + 1, updated_at = now() "+
		"WHERE id = $1 RETURNING "+columns, id, body))
	if err != nil {
		return c, err
	}
	return c, s.publish(ctx, "comment_updated", c)
}

func (s *Store) Vote(ctx context.Context, id int64, delta int) (Comment, error) {
	if delta != 1 && delta != -1 {
		return Comment{}, errDelta
	}
	c, err := scanComment(s.db.QueryRow(ctx, "UPDATE comments "+
		"SET score = score + $2, version = version + 1, updated_at = now() "+
		"WHERE id = $1 RETURNING "+columns, id, delta))
	if err != nil {
		return c, err
	}
	return c, s.publish(ctx, "comment_voted", c)
}

func (s *Store) Seed(ctx context.Context, c Comment) error {
	_, err := s.db.Exec(ctx, "INSERT INTO comments ("+columns+") "+
		"VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) "+
		"ON CONFLICT (id) DO UPDATE SET post_id = EXCLUDED.post_id, author = EXCLUDED.author, "+
		"body = EXCLUDED.body, score = EXCLUDED.score, version = EXCLUDED.version, "+
		"edited = EXCLUDED.edited, created_at = EXCLUDED.created_at, updated_at = EXCLUDED.updated_at",
		c.ID, c.PostID, c.Author, c.Body, c.Score, c.Version, c.Edited, c.CreatedAt, c.UpdatedAt)
	return err
}

func (s *Store) publish(ctx context.Context, kind string, c Comment) error {
	payload, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return s.events.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: map[string]any{"type": kind, "payload": payload},
	}).Err()
}
