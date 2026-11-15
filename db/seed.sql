INSERT INTO comments (post_id, author, body, score, created_at, updated_at)
SELECT
        (g % 50) + 1,
        'user_' || (g % 300),
        'comment number ' || g,
        (g * 7) % 120,
now() - (g || ' minutes')::interval,
now() - (g || ' minutes')::interval
FROM generate_series(1, 2000) AS g;