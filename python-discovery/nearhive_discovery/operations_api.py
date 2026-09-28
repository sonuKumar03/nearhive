"""Small authenticated HTTP API for Python-owned discovery runs."""

import base64
import hashlib
import hmac
import json
import math
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from uuid import UUID

import psycopg


def _decode_user(token: str, secret: str) -> UUID:
    if len(token) > 4096:
        raise ValueError("invalid token")
    try:
        header, payload, signature = token.split(".")

        def decode(part: str) -> bytes:
            return base64.urlsafe_b64decode(part + "=" * (-len(part) % 4))

        if json.loads(decode(header)).get("alg") != "HS256":
            raise ValueError("invalid signing algorithm")
        expected = hmac.new(secret.encode(), f"{header}.{payload}".encode(), hashlib.sha256).digest()
        if not hmac.compare_digest(decode(signature), expected):
            raise ValueError("invalid signature")
        claims = json.loads(decode(payload))
        now = int(time.time())
        if not isinstance(claims.get("exp"), int) or claims["exp"] <= now:
            raise ValueError("expired token")
        if not isinstance(claims.get("nbf"), int) or claims["nbf"] > now:
            raise ValueError("token not yet valid")
        if not isinstance(claims.get("iat"), int) or claims["iat"] > now:
            raise ValueError("invalid issue time")
        return UUID(claims["sub"])
    except (KeyError, TypeError, ValueError, json.JSONDecodeError) as exc:
        raise ValueError("invalid token") from exc


def _validate_location(body: object) -> tuple[float, float, float]:
    if not isinstance(body, dict):
        raise ValueError("request body must be an object")
    values = []
    for name, lower, upper in (("lat", -90, 90), ("lng", -180, 180), ("radius_km", 0, 100)):
        value = body.get(name)
        if isinstance(value, bool) or not isinstance(value, (int, float)) or not math.isfinite(value):
            raise ValueError(f"invalid {name}")
        if not lower <= value <= upper or (name == "radius_km" and value == 0):
            raise ValueError(f"invalid {name}")
        values.append(float(value))
    return values[0], values[1], values[2]


def create_server(
    database_url: str,
    jwt_secret: str,
    host: str = "0.0.0.0",
    port: int = 8090,
    allowed_origin: str = "http://localhost:3000",
) -> ThreadingHTTPServer:
    if not jwt_secret:
        raise ValueError("JWT_SECRET is required for discovery operations")

    class Handler(BaseHTTPRequestHandler):
        def _send(self, status: int, data: dict[str, object], head: bool = False) -> None:
            encoded = json.dumps(data).encode()
            self.send_response(status)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(encoded)))
            self.send_header("Cache-Control", "no-store")
            if self.headers.get("Origin") == allowed_origin:
                self.send_header("Access-Control-Allow-Origin", allowed_origin)
                self.send_header("Vary", "Origin")
            self.end_headers()
            if not head:
                self.wfile.write(encoded)

        def _is_health(self) -> bool:
            return self.path.split("?", 1)[0].strip("/") == "health"

        def _user(self) -> UUID | None:
            if self.headers.get("Origin") not in (None, allowed_origin):
                self._send(403, {"error": "origin not allowed"})
                return None
            scheme, _, token = self.headers.get("Authorization", "").partition(" ")
            if scheme.lower() != "bearer" or not token:
                self._send(401, {"error": "unauthorized"})
                return None
            try:
                return _decode_user(token, jwt_secret)
            except ValueError:
                self._send(401, {"error": "unauthorized"})
                return None

        def _parts(self) -> list[str]:
            return self.path.split("?", 1)[0].strip("/").split("/")

        def _job_id(self, parts: list[str]) -> UUID | None:
            try:
                return UUID(parts[4])
            except (ValueError, IndexError):
                self._send(404, {"error": "not found"})
                return None

        def do_OPTIONS(self) -> None:
            if self.headers.get("Origin") != allowed_origin:
                self._send(403, {"error": "origin not allowed"})
                return
            self.send_response(204)
            self.send_header("Access-Control-Allow-Origin", allowed_origin)
            self.send_header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
            self.send_header("Access-Control-Allow-Headers", "Authorization, Content-Type")
            self.send_header("Content-Length", "0")
            self.end_headers()

        def do_POST(self) -> None:
            user_id = self._user()
            if user_id is None:
                return
            parts = self._parts()
            if parts[:4] != ["api", "v1", "discovery", "jobs"]:
                self._send(404, {"error": "not found"})
                return
            if len(parts) == 4:
                try:
                    length = int(self.headers.get("Content-Length", "0"))
                    if not 0 < length <= 4096:
                        raise ValueError("invalid request size")
                    lat, lng, radius = _validate_location(json.loads(self.rfile.read(length)))
                except (ValueError, json.JSONDecodeError):
                    self._send(400, {"error": "invalid location or radius"})
                    return
                try:
                    with psycopg.connect(database_url, autocommit=True) as db:
                        row = db.execute(
                            """INSERT INTO discovery_jobs (user_id, lat, lng, radius_km)
                               SELECT id, %s, %s, %s FROM users WHERE id = %s RETURNING id""",
                            (lat, lng, radius, user_id),
                        ).fetchone()
                except psycopg.Error:
                    self._send(503, {"error": "database unavailable"})
                    return
                if row is None:
                    self._send(401, {"error": "unauthorized"})
                    return
                self._send(201, {"id": str(row[0]), "status": "queued"})
                return
            if len(parts) == 6 and parts[5] == "cancel":
                job_id = self._job_id(parts)
                if job_id is not None:
                    self._status(user_id, job_id, cancel=True)
                return
            self._send(404, {"error": "not found"})

        def do_GET(self) -> None:
            if self._is_health():
                self._send(200, {"status": "ok"})
                return
            user_id = self._user()
            if user_id is None:
                return
            parts = self._parts()
            if parts[:4] != ["api", "v1", "discovery", "jobs"]:
                self._send(404, {"error": "not found"})
                return
            if len(parts) == 4:
                try:
                    with psycopg.connect(database_url) as db:
                        rows = db.execute(
                            """SELECT id, status, lat, lng, radius_km, created_at
                               FROM discovery_jobs WHERE user_id = %s
                               ORDER BY created_at DESC LIMIT 20""",
                            (user_id,),
                        ).fetchall()
                except psycopg.Error:
                    self._send(503, {"error": "database unavailable"})
                    return
                statuses = {"pending": "queued", "running": "in_progress", "partial": "completed"}
                self._send(200, {"jobs": [
                    {"id": str(id), "status": statuses.get(status, status), "lat": lat,
                     "lng": lng, "radius_km": radius, "created_at": created.isoformat()}
                    for id, status, lat, lng, radius, created in rows
                ]})
                return
            if len(parts) != 5:
                self._send(404, {"error": "not found"})
                return
            job_id = self._job_id(parts)
            if job_id is not None:
                self._status(user_id, job_id)

        def do_HEAD(self) -> None:
            if self._is_health():
                self._send(200, {"status": "ok"}, head=True)
                return
            self._send(404, {"error": "not found"}, head=True)

        def _status(self, user_id: UUID, job_id: UUID, cancel: bool = False) -> None:
            try:
                with psycopg.connect(database_url, autocommit=True) as db:
                    if cancel:
                        db.execute(
                            """UPDATE discovery_jobs SET status = 'cancelled', updated_at = NOW()
                               WHERE id = %s AND user_id = %s AND status IN ('pending', 'running')""",
                            (job_id, user_id),
                        )
                    row = db.execute(
                        "SELECT status FROM discovery_jobs WHERE id = %s AND user_id = %s",
                        (job_id, user_id),
                    ).fetchone()
            except psycopg.Error:
                self._send(503, {"error": "database unavailable"})
                return
            if row is None:
                self._send(404, {"error": "not found"})
                return
            status = {
                "pending": "queued", "running": "in_progress", "partial": "completed"
            }.get(row[0], row[0])
            self._send(200, {"id": str(job_id), "status": status})

    return ThreadingHTTPServer((host, port), Handler)
