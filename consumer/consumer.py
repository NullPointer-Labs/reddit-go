import json
import os
import sys
import time
from datetime import datetime

import redis

STREAM = "comments.events"
r = redis.Redis.from_url(os.environ["REDIS_URL"], decode_responses=True)
name = os.environ.get("CONSUMER_NAME", "consumer")


def parse(payload: dict) -> dict:
    return {
        "id": int(payload["id"]),
        "score": int(payload["score"]),
        "created_at": datetime.strptime(payload["created_at"], "%Y-%m-%dT%H:%M:%S.%f%z"),
    }


def on_created(evt):
    pass


def on_updated(evt):
    pass


def on_voted(evt):
    pass


HANDLERS = {
    "comment_created": on_created,
    "comment_updated": on_updated,
    "comment_voted": on_voted,
}


def main():
    last_id = "$"
    stats = {"processed": 0, "skipped": 0, "failed": 0}
    print(f"[{name}] listening on {STREAM}", flush=True)
    last_report = time.monotonic()
    while True:
        resp = r.xread({STREAM: last_id}, block=1000, count=100)
        if time.monotonic() - last_report >= 5:
            print(f"[{name}] processed={stats['processed']} skipped={stats['skipped']} failed={stats['failed']}", flush=True)
            last_report = time.monotonic()
        for _, entries in resp or []:
            for entry_id, fields in entries:
                last_id = entry_id
                handler = HANDLERS.get(fields.get("type"))
                if handler is None:
                    stats["skipped"] += 1
                    continue
                try:
                    handler(parse(json.loads(fields["payload"])))
                    stats["processed"] += 1
                except (KeyError, ValueError) as e:
                    stats["failed"] += 1
                    print(f"[{name}] DESERIALIZE FAIL entry={entry_id}: {e}", file=sys.stderr, flush=True)


if __name__ == "__main__":
    main()