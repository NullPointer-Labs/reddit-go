# reddit-go

Rebuilding Reddit's comments migration from Python to Go, from scratch and with the system live.

A Python monolith (Flask + Postgres + Redis + an event consumer) gets a Go service running next to it. Before Go is allowed to answer a single user, it has to prove it behaves exactly like the old code: **tap compare** for reads, **sister datastores** for writes. Only then does the router switch over, with rollback one `curl` away.

Every file was typed live in the video, bugs included.

Video (pt-BR): https://youtu.be/FZo7ALwJAAk

## Architecture

```
                 ┌──────────────┐
  k6 / user ───▶ │ router :8080 │ ── GO_PERCENT % ──┐
                 └──────┬───────┘                    │
                        │                            ▼
                        ▼                    ┌──────────────────┐
                ┌───────────────┐  compare   │ comments-go :8081 │
                │ legacy (Flask)│ ◀────────▶ │ shadow / primary  │
                └──┬────────┬───┘            └──┬────────┬───────┘
                   │        │                   │        │
                 pg       redis ──▶ consumer   pg-sister redis-sister ──▶ consumer-sister
              (production)                     (only Go writes here)
```

- **Reads:** the router sends a share of the traffic to Go. Go builds its answer, asks the legacy service the same question and compares both. The user always gets the legacy answer, and every difference becomes a `MISMATCH` log line.
- **Writes:** legacy writes to production; Go repeats the write on its own sister copies with the same ID, then both sides are compared field by field.
- **Scoreboard:** `GET /admin/stats` on the Go service counts reads and writes compared, mismatches and races.

## Steps

Each step is a branch, so you can check out the exact state of the code at that point in the video. `main` is the final state.

| Branch | Step |
|---|---|
| `step-1-legacy` | The Python monolith from scratch, Docker Compose and the k6 load test |
| `step-2-readings` | The router and the Go service in tap compare |
| `step-3-readings-ok` | Fixing the reads until they match |
| `step-4-writings` | Writes with sister datastores |
| `step-5-running` | Clock and race fixes, then step 5.1: the silent consumer bug |
| `step-6-turning-point` | The switch: Go becomes primary |

## Running it

Requirements: Docker with Compose, and [k6](https://k6.io/) for the load test.

```bash
docker compose up --build
```

Load test (defaults to `http://localhost:8080`, 300 req/s for 60 s):

```bash
k6 run -e RATE=600 -e DURATION=60s k6/load.js
```

Move traffic to Go and watch the scoreboard:

```bash
curl -X POST 'localhost:8080/admin/percent?v=100'
curl localhost:8081/admin/stats
```

The switch, and the rollback:

```bash
curl -X POST 'localhost:8081/admin/mode?v=primary'
curl -X POST 'localhost:8080/admin/percent?v=0'
```

## Numbers from the lab

Measured on a single machine with synthetic load, so read them as a comparison of concurrency models, not of languages.

| | |
|---|---|
| Legacy at 600 req/s | p99 230 ms |
| Legacy at 1200 req/s | p99 2.13 s, 10,728 dropped requests |
| Reads after the fixes | 4,857 compared, 0 mismatches |
| Writes before / after | 1,149 of 1,149 mismatched → 0, with 12 races in 1,158 |
| The silent bug | 2,307 events skipped by the sister consumer |
| Go as primary | avg 4.79 ms → 1.32 ms, 4,000 req/s, 0 failed, p99 6.93 ms |
| Memory | 17.8 MiB (Go) vs 203.7 MiB (legacy) |

## Based on

Reddit Engineering's real migration of the comments model from their Python monolith to Go: [InfoQ write-up](https://www.infoq.com/news/2025/11/reddit-comments-go-migration).
