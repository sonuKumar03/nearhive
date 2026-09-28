import base64
import hashlib
import hmac
import http.client
import json
import os
import threading
import time
import uuid

import psycopg
import pytest

from nearhive_discovery.operations_api import create_server


def _token(user_id: uuid.UUID, secret: str) -> str:
    def encode(value: dict) -> str:
        return base64.urlsafe_b64encode(json.dumps(value, separators=(",", ":")).encode()).rstrip(b"=").decode()

    now = int(time.time())
    signed = ".".join(
        (encode({"alg": "HS256", "typ": "JWT"}), encode({"sub": str(user_id), "iat": now, "nbf": now, "exp": now + 300}))
    )
    signature = base64.urlsafe_b64encode(hmac.new(secret.encode(), signed.encode(), hashlib.sha256).digest()).rstrip(b"=").decode()
    return f"{signed}.{signature}"


@pytest.mark.skipif(not os.getenv("NEARHIVE_TEST_DATABASE_URL"), reason="isolated test database required")
def test_authenticated_discovery_lifecycle() -> None:
    database_url = os.environ["NEARHIVE_TEST_DATABASE_URL"]
    secret = "operations-api-test-secret"
    user_id = uuid.uuid4()
    with psycopg.connect(database_url, autocommit=True) as db:
        db.execute(
            "INSERT INTO users (id, email, password) VALUES (%s, %s, %s)",
            (user_id, f"discovery-api-{user_id.hex}@example.com", "hash"),
        )

    server = create_server(database_url, secret, host="127.0.0.1", port=0)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    conn = http.client.HTTPConnection("127.0.0.1", server.server_port, timeout=5)
    try:
        conn.request("GET", "/health")
        response = conn.getresponse()
        assert response.status == 200
        assert json.loads(response.read()) == {"status": "ok"}

        conn.request("POST", "/api/v1/discovery/jobs", body='{"lat":12.97,"lng":77.59,"radius_km":10}')
        response = conn.getresponse()
        assert response.status == 401
        response.read()

        headers = {"Authorization": f"Bearer {_token(user_id, secret)}", "Content-Type": "application/json"}
        conn.request("POST", "/api/v1/discovery/jobs", body='{"lat":12.97,"lng":77.59,"radius_km":10}', headers=headers)
        response = conn.getresponse()
        assert response.status == 201
        created = json.loads(response.read())
        assert created["status"] == "queued"

        conn.request("GET", "/api/v1/discovery/jobs", headers=headers)
        response = conn.getresponse()
        assert response.status == 200
        assert created["id"] in [job["id"] for job in json.loads(response.read())["jobs"]]

        path = f"/api/v1/discovery/jobs/{created['id']}"
        conn.request("GET", path, headers=headers)
        response = conn.getresponse()
        assert response.status == 200
        assert json.loads(response.read()) == created

        conn.request("POST", f"{path}/cancel", headers=headers)
        response = conn.getresponse()
        assert response.status == 200
        assert json.loads(response.read())["status"] == "cancelled"
    finally:
        conn.close()
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)
        with psycopg.connect(database_url, autocommit=True) as db:
            db.execute("DELETE FROM users WHERE id = %s", (user_id,))
