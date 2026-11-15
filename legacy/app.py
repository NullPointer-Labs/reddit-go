import json
import os
from datetime import datetime, timezone

import redis
from flask import Flask, jsonify, request
from sqlalchemy import BigInteger, Boolean, DateTime, Integer, Text, create_engine, select
from sqlalchemy.orm import DeclarativeBase, Mapped, Session, mapped_column

engine = create_engine(os.environ["DATABASE_URL"], pool_size=5, max_overflow=0)
events = redis.Redis.from_url(os.environ["REDIS_URL"])
STREAM = "comments.events"

app = Flask(__name__)


class Base(DeclarativeBase):
    pass


class Comment(Base):
    __tablename__ = "comments"

    id: Mapped[int] = mapped_column(BigInteger, primary_key=True)
    post_id: Mapped[int] = mapped_column(BigInteger)
    author: Mapped[str] = mapped_column(Text)
    body: Mapped[str] = mapped_column(Text)
    score: Mapped[int] = mapped_column(Integer, default=0)
    version: Mapped[int] = mapped_column(Integer, default=1)
    edited: Mapped[bool] = mapped_column(Boolean, default=False)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True))
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True))


def ts(dt: datetime) -> str:
    return dt.astimezone(timezone.utc).isoformat(timespec="microseconds")


def to_dict(c: Comment) -> dict:
    return {
        "id": c.id,
        "post_id": c.post_id,
        "author": c.author,
        "body": c.body,
        "score": c.score,
        "version": c.version,
        "edited": c.edited,
        "created_at": ts(c.created_at),
        "updated_at": ts(c.updated_at),
    }


def publish(kind: str, c: Comment) -> None:
    events.xadd(STREAM, {"type": kind, "payload": json.dumps(to_dict(c))})


def error(status: int, msg: str):
    return jsonify({"error": msg}), status


@app.get("/comments/<int:comment_id>")
def get_comment(comment_id: int):
    with Session(engine) as s:
        c = s.get(Comment, comment_id)
        if c is None:
            return error(404, "not found")
        return jsonify(to_dict(c))


@app.get("/posts/<int:post_id>/comments")
def list_comments(post_id: int):
    with Session(engine) as s:
        rows = s.scalars(
            select(Comment)
            .where(Comment.post_id == post_id)
            .order_by(Comment.score.desc(), Comment.id)
            .limit(50)
        ).all()
        return jsonify([to_dict(c) for c in rows])


@app.post("/comments")
def create_comment():
    data = request.get_json(silent=True) or {}
    body = (data.get("body") or "").strip()
    if not body:
        return error(400, "body required")
    if not isinstance(data.get("post_id"), int) or not data.get("author"):
        return error(400, "post_id and author required")
    now = datetime.now(timezone.utc)
    with Session(engine) as s:
        c = Comment(
            post_id=data["post_id"], author=data["author"], body=body,
            score=0, version=1, edited=False, created_at=now, updated_at=now,
        )
        s.add(c)
        s.commit()
        publish("comment_created", c)
        return jsonify(to_dict(c)), 201


@app.patch("/comments/<int:comment_id>")
def update_comment(comment_id: int):
    data = request.get_json(silent=True) or {}
    body = (data.get("body") or "").strip()
    if not body:
        return error(400, "body required")
    with Session(engine) as s:
        c = s.get(Comment, comment_id, with_for_update=True)
        if c is None:
            return error(404, "not found")
        c.body = body
        c.edited = True
        c.version += 1
        c.updated_at = datetime.now(timezone.utc)
        s.commit()
        publish("comment_updated", c)
        return jsonify(to_dict(c))


@app.post("/comments/<int:comment_id>/vote")
def vote(comment_id: int):
    data = request.get_json(silent=True) or {}
    delta = data.get("delta")
    if delta not in (1, -1):
        return error(400, "delta must be 1 or -1")
    with Session(engine) as s:
        c = s.get(Comment, comment_id, with_for_update=True)
        if c is None:
            return error(404, "not found")
        c.score += delta
        c.version += 1
        c.updated_at = datetime.now(timezone.utc)
        s.commit()
        publish("comment_voted", c)
        return jsonify(to_dict(c))