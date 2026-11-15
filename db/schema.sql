CREATE TABLE IF NOT EXISTS comments (
    id          BIGSERIAL PRIMARY KEY,
    post_id     BIGINT      NOT NULL,
    author      TEXT        NOT NULL,
    body        TEXT        NOT NULL,
    score       INT         NOT NULL DEFAULT 0,
    version     INT         NOT NULL DEFAULT 1,
    edited      BOOLEAN     NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS comments_post_score_idx ON comments (post_id, score DESC, id);