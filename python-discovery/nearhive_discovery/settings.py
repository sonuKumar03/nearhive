import os
import uuid
from dataclasses import dataclass


@dataclass(frozen=True)
class Settings:
    database_url: str = os.getenv(
        "DATABASE_URL",
        "postgres://postgres:postgres@localhost:5432/nearhive?sslmode=disable",
    )
    jwt_secret: str = os.getenv("JWT_SECRET", "")
    api_port: int = int(os.getenv("PORT", os.getenv("NEARHIVE_DISCOVERY_API_PORT", "8090")))
    allowed_origin: str = os.getenv("NEARHIVE_ALLOWED_ORIGIN", "http://localhost:3000")
    worker_id: str = os.getenv("NEARHIVE_WORKER_ID", f"worker-{uuid.uuid4().hex[:8]}")
    lease_seconds: int = int(os.getenv("NEARHIVE_LEASE_SECONDS", "60"))
    heartbeat_interval_seconds: float = float(os.getenv("NEARHIVE_HEARTBEAT_INTERVAL", "15.0"))
    poll_interval_seconds: float = float(os.getenv("NEARHIVE_POLL_INTERVAL", "2.0"))
    playwright_contexts: int = int(os.getenv("PLAYWRIGHT_CONTEXTS", "2"))
    max_company_sites: int = int(os.getenv("NEARHIVE_MAX_COMPANY_SITES", "50"))
    geocoder_url: str = os.getenv(
        "NEARHIVE_GEOCODER_URL", "https://nominatim.openstreetmap.org/search"
    )
    geocoder_provider: str = os.getenv("NEARHIVE_GEOCODER_PROVIDER", "nominatim")
    max_job_geocodes: int = int(os.getenv("NEARHIVE_MAX_JOB_GEOCODES", "50"))



settings = Settings()
