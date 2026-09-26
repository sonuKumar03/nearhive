from collections.abc import AsyncIterator
from typing import Protocol, runtime_checkable

from nearhive_discovery.contracts import DiscoveryJob, EvidenceBatch


@runtime_checkable
class SourceAdapter(Protocol):
    """Protocol defining the standard interface for NearHive discovery source adapters."""

    name: str
    source_family: str

    def run(self, job: DiscoveryJob) -> AsyncIterator[EvidenceBatch]:
        """Executes discovery against the source, streaming evidence batches."""
        ...


class BaseSourceAdapter:
    """Convenience base class for implementing SourceAdapter."""

    name: str = ""
    source_family: str = ""

    async def run(self, job: DiscoveryJob) -> AsyncIterator[EvidenceBatch]:
        raise NotImplementedError
        yield  # make it an async generator
