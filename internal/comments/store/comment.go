package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Comment struct {
	ID        int64     `json:"id"`
	PostID    int64     `json:"post_id"`
	Author    string    `json:"author"`
	Body      string    `json:"body"`
	Score     int       `json:"score"`
	Version   int       `json:"version"`
	Edited    bool      `json:"edited"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func pyTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000000-07:00")
}

func (c Comment) MarshalJSON() ([]byte, error) {
	type alias Comment
	return json.Marshal(struct {
		alias
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}{alias(c), pyTime(c.CreatedAt), pyTime(c.UpdatedAt)})
}

type Store struct {
	db     *pgxpool.Pool
	events *redis.Client
}

func Open(ctx context.Context, dbURL, redisURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return nil, err
	}
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	return &Store{db: pool, events: redis.NewClient(opts)}, nil
}

type Error struct {
	Status int
	Msg    string
}

func (e *Error) Error() string { return e.Msg }

var errNotFound = &Error{404, "not found"}

const columns = "id, post_id, author, body, score, version, edited, created_at, updated_at"

func scanComment(row pgx.Row) (Comment, error) {
	var c Comment
	err := row.Scan(&c.ID, &c.PostID, &c.Author, &c.Body, &c.Score, &c.Version, &c.Edited, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, errNotFound
	}
	return c, err
}

func (s *Store) Get(ctx context.Context, id int64) (Comment, error) {
	return scanComment(s.db.QueryRow(ctx, "SELECT "+columns+" FROM comments WHERE id = $1", id))
}

func (s *Store) ListByPost(ctx context.Context, postID int64) ([]Comment, error) {
	rows, err := s.db.Query(ctx,
		"SELECT "+columns+" FROM comments WHERE post_id = $1 ORDER BY score DESC, id LIMIT 50", postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Comment{}
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
