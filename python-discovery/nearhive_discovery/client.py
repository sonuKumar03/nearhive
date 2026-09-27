from typing import Any
import httpx

from nearhive_discovery.contracts import (
    BatchResult,
    EvidenceBatch,
    batch_to_dict,
)


class IngestionError(Exception):
    """Base exception for ingestion client errors."""
    pass


class AuthenticationError(IngestionError):
    """Raised when worker token authentication fails (HTTP 401)."""
    pass


class ValidationError(IngestionError):
    """Raised when batch payload validation fails (HTTP 400)."""
    pass


class PayloadTooLargeError(IngestionError):
    """Raised when batch payload exceeds size limit (HTTP 413)."""
    pass


class IngestionClient:
    def __init__(
        self,
        base_url: str,
        worker_token: str,
        client: httpx.Client | None = None,
        timeout: float = 30.0,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        self.worker_token = worker_token
        self._owned_client = client is None
        self.client = client or httpx.Client(timeout=timeout)

    def submit(self, batch: EvidenceBatch) -> BatchResult:
        url = f"{self.base_url}/api/v1/internal/discovery/batches"
        headers = {
            "X-NearHive-Worker-Token": self.worker_token,
            "Content-Type": "application/json",
        }
        payload = batch_to_dict(batch)

        try:
            resp = self.client.post(url, json=payload, headers=headers)
        except httpx.RequestError as exc:
            raise IngestionError(f"Network error submitting batch: {exc}") from exc

        if resp.status_code == 200:
            return BatchResult.from_dict(resp.json())
        elif resp.status_code == 401:
            raise AuthenticationError(f"Authentication failed (401): {resp.text}")
        elif resp.status_code == 400:
            raise ValidationError(f"Batch validation failed (400): {resp.text}")
        elif resp.status_code == 413:
            raise PayloadTooLargeError(f"Batch payload too large (413): {resp.text}")
        else:
            raise IngestionError(
                f"Ingestion failed with status {resp.status_code}: {resp.text}"
            )

    def close(self) -> None:
        if self._owned_client:
            self.client.close()

    def __enter__(self) -> "IngestionClient":
        return self

    def __exit__(self, exc_type: Any, exc_val: Any, exc_tb: Any) -> None:
        self.close()
