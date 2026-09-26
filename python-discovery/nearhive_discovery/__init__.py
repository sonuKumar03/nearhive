"""NearHive Python Discovery package."""

from nearhive_discovery.client import (
    AuthenticationError,
    IngestionClient,
    IngestionError,
    PayloadTooLargeError,
    ValidationError,
)
from nearhive_discovery.contracts import (
    CONTRACT_VERSION,
    BatchRecordResult,
    BatchResult,
    CompanyEvidence,
    DiscoveryBatch,
    DiscoveryJob,
    DiscoverySourceRun,
    DiscoveryStatus,
    EvidenceBatch,
    PresenceType,
    PublicationState,
    TechnicalJobEvidence,
    WorkArrangement,
    batch_to_dict,
    batch_to_json,
)
from nearhive_discovery.queue import (
    claim_job,
    finish_job,
    heartbeat,
    is_cancelled,
    record_source_run,
)
from nearhive_discovery.settings import Settings, settings
from nearhive_discovery.worker import Source, Worker

__all__ = [
    "CONTRACT_VERSION",
    "AuthenticationError",
    "BatchRecordResult",
    "BatchResult",
    "CompanyEvidence",
    "DiscoveryBatch",
    "DiscoveryJob",
    "DiscoverySourceRun",
    "DiscoveryStatus",
    "EvidenceBatch",
    "IngestionClient",
    "IngestionError",
    "PayloadTooLargeError",
    "PresenceType",
    "PublicationState",
    "Settings",
    "Source",
    "TechnicalJobEvidence",
    "ValidationError",
    "WorkArrangement",
    "Worker",
    "batch_to_dict",
    "batch_to_json",
    "claim_job",
    "finish_job",
    "heartbeat",
    "is_cancelled",
    "record_source_run",
    "settings",
]

__version__ = "0.1.0"
