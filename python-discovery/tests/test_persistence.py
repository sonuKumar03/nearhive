import os
import uuid
from dataclasses import replace

import psycopg
import pytest

from nearhive_discovery.contracts import CompanyEvidence, EvidenceBatch, TechnicalJobEvidence, WorkArrangement
from nearhive_discovery.persistence import PostgresPersistence
from nearhive_discovery.worker import Worker


@pytest.mark.skipif(
    not os.getenv("NEARHIVE_TEST_DATABASE_URL"),
    reason="set NEARHIVE_TEST_DATABASE_URL to a migrated disposable database",
)
def test_batch_persistence_is_canonical_and_idempotent() -> None:
    database_url = os.environ["NEARHIVE_TEST_DATABASE_URL"]
    suffix = uuid.uuid4().hex
    source = f"persistence-test-{suffix}"
    domain = f"{suffix}.example"
    other_domain = f"other-{suffix}.example"
    job_id = uuid.uuid4()
    batch = EvidenceBatch(
        contract_version=1,
        discovery_job_id=job_id,
        source=source,
        source_family="official_site",
        observed_at=__import__("datetime").datetime.now(__import__("datetime").timezone.utc),
        companies=[
            CompanyEvidence(
                name="Persistence Test Company",
                source_record_id=suffix,
                domain=domain,
                address="Test Road",
                lat=12.97,
                lng=77.59,
            )
        ],
        jobs=[
            TechnicalJobEvidence(
                company_name="Persistence Test Company",
                company_domain=domain,
                title="Software Engineer",
                source_job_id=suffix,
                canonical_url=f"https://{domain}/jobs/{suffix}",
                lat=12.98,
                lng=77.60,
                technical_classification="software",
            )
        ],
    )

    try:
        with PostgresPersistence(database_url) as persistence:
            first = persistence.persist(batch)
            second = persistence.persist(batch)
            persistence.persist(
                replace(
                    batch,
                    companies=[replace(batch.companies[0], source_record_id=f"other-{suffix}", domain=other_domain)],
                    jobs=[],
                )
            )
        worker = Worker(
            db_url=database_url,
            sources=[],
            playwright_pool=object(),
            location_resolver=False,
        )
        from_worker = worker._submit_batch(batch)
        with psycopg.connect(database_url) as conn:
            company_count = conn.execute(
                "SELECT count(*) FROM companies WHERE domain IN (%s, %s)", (domain, other_domain)
            ).fetchone()[0]
            job_count = conn.execute(
                "SELECT count(*) FROM technical_job_postings WHERE source = %s AND source_job_id = %s",
                (source, suffix),
            ).fetchone()[0]
            job_location = conn.execute(
                """SELECT jl.latitude, jl.longitude FROM job_locations jl
                   JOIN technical_job_postings j ON j.id = jl.job_id
                   WHERE j.source = %s AND j.source_job_id = %s""",
                (source, suffix),
            ).fetchone()
            sighting_count = conn.execute(
                "SELECT count(*) FROM sightings WHERE source = %s",
                (source,),
            ).fetchone()[0]

        assert first.accepted_companies == second.accepted_companies == from_worker.accepted_companies == 1
        assert first.accepted_jobs == second.accepted_jobs == from_worker.accepted_jobs == 1
        assert (company_count, job_count, sighting_count) == (2, 1, 2)
        assert job_location == (12.98, 77.60)
    finally:
        with psycopg.connect(database_url) as conn:
            conn.execute("DELETE FROM sightings WHERE source = %s", (source,))
            conn.execute("DELETE FROM companies WHERE domain IN (%s, %s)", (domain, other_domain))


def test_persistence_rejects_invalid_coordinates_before_connecting() -> None:
    batch = EvidenceBatch(
        contract_version=1,
        discovery_job_id=uuid.uuid4(),
        source="test",
        source_family="official_site",
        observed_at=__import__("datetime").datetime.now(__import__("datetime").timezone.utc),
        companies=[CompanyEvidence(name="Invalid Co", lat=91, lng=0)],
    )

    with pytest.raises(ValueError, match="invalid coordinates"):
        PostgresPersistence.validate(batch)

    remote_batch = EvidenceBatch(
        contract_version=1,
        discovery_job_id=uuid.uuid4(),
        source="test",
        source_family="job_ats",
        observed_at=batch.observed_at,
        jobs=[
            TechnicalJobEvidence(
                company_name="Remote Co",
                title="Software Engineer",
                work_arrangement=WorkArrangement.REMOTE,
                lat=12.97,
                lng=77.59,
            )
        ],
    )
    with pytest.raises(ValueError, match="remote job cannot have spatial coordinates"):
        PostgresPersistence.validate(remote_batch)
