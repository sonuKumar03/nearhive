import os
import uuid
from dataclasses import dataclass


@dataclass(frozen=True)
class Settings:
    database_url: str = os.getenv(
        "DATABASE_URL",
        "postgres://nearhive:password@localhost:5432/nearhive?sslmode=disable",
    )
    api_base_url: str = os.getenv("NEARHIVE_API_URL", "http://localhost:8080")
    worker_token: str = os.getenv("NEARHIVE_WORKER_TOKEN") or os.getenv("DISCOVERY_WORKER_TOKEN") or "dev-worker-token"
    worker_id: str = os.getenv("NEARHIVE_WORKER_ID", f"worker-{uuid.uuid4().hex[:8]}")
    lease_seconds: int = int(os.getenv("NEARHIVE_LEASE_SECONDS", "60"))
    heartbeat_interval_seconds: float = float(os.getenv("NEARHIVE_HEARTBEAT_INTERVAL", "15.0"))
    poll_interval_seconds: float = float(os.getenv("NEARHIVE_POLL_INTERVAL", "2.0"))
    playwright_contexts: int = int(os.getenv("PLAYWRIGHT_CONTEXTS", "2"))
    max_company_sites: int = int(os.getenv("NEARHIVE_MAX_COMPANY_SITES", "50"))



settings = Settings()
