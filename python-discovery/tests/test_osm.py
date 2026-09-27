import json
from uuid import uuid4
import pytest

from nearhive_discovery.contracts import DiscoveryJob, DiscoveryStatus
from nearhive_discovery.sources.osm import OpenStreetMapSource


@pytest.mark.asyncio
async def test_osm_source_properties():
    src = OpenStreetMapSource()
    assert src.name == "osm_dataset"
    assert src.source_family == "open_dataset"


@pytest.mark.asyncio
async def test_osm_source_parsing():
    fake_response = {
        "elements": [
            {
                "type": "node",
                "id": 123456,
                "lat": 17.4485,
                "lon": 78.3741,
                "tags": {
                    "name": "Acme Tech Solutions",
                    "office": "it",
                    "website": "https://www.acmetech.io/careers",
                    "phone": "+91-40-12345678",
                    "addr:street": "Hitec City Main Rd",
                    "addr:city": "Hyderabad",
                    "addr:postcode": "500081",
                },
            },
            {
                "type": "way",
                "id": 987654,
                "center": {"lat": 17.4390, "lon": 78.3800},
                "tags": {
                    "brand": "InnoSoft Park",
                    "office": "company",
                },
            },
            {
                "type": "node",
                "id": 111111,
                "lat": 17.4000,
                "lon": 78.3500,
                "tags": {
                    # No name, brand, or operator -> should be skipped
                    "office": "it",
                },
            },
        ]
    }

    async def mock_fetcher(endpoint: str, query: str) -> str:
        assert "around:" in query
        assert "17.44" in query
        return json.dumps(fake_response)

    src = OpenStreetMapSource(fetcher=mock_fetcher, batch_size=10)
    job = DiscoveryJob(
        id=uuid4(),
        user_id=uuid4(),
        status=DiscoveryStatus.PENDING,
        lat=17.44,
        lng=78.38,
        radius_km=10.0,
    )

    batches = []
    async for b in src.run(job):
        batches.append(b)

    assert len(batches) == 1
    batch = batches[0]
    assert batch.source == "osm_dataset"
    assert batch.source_family == "open_dataset"
    assert len(batch.companies) == 2

    c1 = batch.companies[0]
    assert c1.name == "Acme Tech Solutions"
    assert c1.domain == "acmetech.io"
    assert c1.lat == 17.4485
    assert c1.lng == 78.3741
    assert c1.phone == "+91-40-12345678"
    assert "Hitec City Main Rd" in c1.address
    assert c1.source_record_id == "osm:node:123456"
    assert c1.evidence_url == "https://www.openstreetmap.org/node/123456"

    c2 = batch.companies[1]
    assert c2.name == "InnoSoft Park"
    assert c2.domain is None
    assert c2.lat == 17.4390
    assert c2.lng == 78.3800
    assert c2.source_record_id == "osm:way:987654"
