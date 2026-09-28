import os
import uuid
from dataclasses import replace
from datetime import datetime, timezone

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


@pytest.mark.skipif(
    not os.getenv("NEARHIVE_TEST_DATABASE_URL"),
    reason="set NEARHIVE_TEST_DATABASE_URL to a migrated disposable database",
)
def test_provenance_rows_written_and_idempotent() -> None:
    database_url = os.environ["NEARHIVE_TEST_DATABASE_URL"]
    suffix = uuid.uuid4().hex
    source = f"provenance-test-{suffix}"
    domain = f"{suffix}.example"
    batch = EvidenceBatch(
        contract_version=1,
        discovery_job_id=uuid.uuid4(),
        source=source,
        source_family="official_site",
        observed_at=datetime.now(timezone.utc),
        companies=[
            CompanyEvidence(
                name="Provenance Test Company",
                source_record_id=suffix,
                domain=domain,
                address="Test Road",
                lat=12.97,
                lng=77.59,
            )
        ],
        jobs=[
            TechnicalJobEvidence(
                company_name="Provenance Test Company",
                company_domain=domain,
                title="Software Engineer",
                source_job_id=suffix,
                canonical_url=f"https://{domain}/jobs/{suffix}",
                technical_classification="software",
            )
        ],
    )
    hash_keyed_batch = replace(
        batch, companies=[replace(batch.companies[0], source_record_id=None)], jobs=[]
    )

    try:
        with PostgresPersistence(database_url) as persistence:
            persistence.persist(batch)
            with psycopg.connect(database_url) as conn:
                company_seen_first = conn.execute(
                    """SELECT sr.last_seen_at FROM source_records sr
                       JOIN data_sources ds ON ds.id = sr.data_source_id
                       WHERE ds.slug = %s AND sr.record_type = 'company' AND sr.external_id = %s""",
                    (source, suffix),
                ).fetchone()[0]
            persistence.persist(batch)
            persistence.persist(hash_keyed_batch)
            persistence.persist(hash_keyed_batch)

        with psycopg.connect(database_url) as conn:
            data_sources = conn.execute(
                "SELECT id, name, source_type, source_family FROM data_sources WHERE slug = %s",
                (source,),
            ).fetchall()
            data_source_id = data_sources[0][0]
            runs = conn.execute(
                """SELECT status, accepted_count, rejected_count, finished_at IS NOT NULL,
                          adapter_version FROM ingestion_runs
                   WHERE data_source_id = %s ORDER BY started_at""",
                (data_source_id,),
            ).fetchall()
            records = conn.execute(
                """SELECT record_type, external_id, count(*), max(last_seen_at),
                          max(normalized_payload_hash) IS NOT NULL
                   FROM source_records WHERE data_source_id = %s
                   GROUP BY record_type, external_id""",
                (data_source_id,),
            ).fetchall()
            job_record = conn.execute(
                """SELECT normalized_payload->>'title', source_url FROM source_records
                   WHERE data_source_id = %s AND record_type = 'job' AND external_id = %s""",
                (data_source_id, suffix),
            ).fetchone()
            company_links = conn.execute(
                """SELECT count(*) FROM company_source_links csl
                   JOIN source_records sr ON sr.id = csl.source_record_id
                   WHERE sr.data_source_id = %s AND csl.relation = 'evidence' AND csl.confidence = 1.0""",
                (data_source_id,),
            ).fetchone()[0]
            job_links = conn.execute(
                """SELECT count(*) FROM job_source_links jsl
                   JOIN source_records sr ON sr.id = jsl.source_record_id
                   WHERE sr.data_source_id = %s AND jsl.relation = 'evidence' AND jsl.confidence = 1.0""",
                (data_source_id,),
            ).fetchone()[0]

        company_seen_second = next(
            row[3] for row in records if row[0] == "company" and row[1] == suffix
        )
        assert len(data_sources) == 1
        assert data_sources[0][1:] == (source, "official_site", "official_site")
        assert [run[0] for run in runs] == ["completed"] * 4
        assert [run[1] for run in runs] == [2, 2, 1, 1]
        assert all(run[2] == 0 and run[3] and run[4] == "v1" for run in runs)
        assert {(row[0], row[1]): row[2] for row in records} == {
            ("company", suffix): 1,
            ("company", None): 1,
            ("job", suffix): 1,
        }
        assert all(row[4] for row in records)
        assert job_record == ("Software Engineer", f"https://{domain}/jobs/{suffix}")
        assert (company_links, job_links) == (2, 1)
        assert company_seen_second > company_seen_first
    finally:
        with psycopg.connect(database_url) as conn:
            conn.execute(
                "DELETE FROM source_records WHERE data_source_id IN (SELECT id FROM data_sources WHERE slug = %s)",
                (source,),
            )
            conn.execute(
                "DELETE FROM ingestion_runs WHERE data_source_id IN (SELECT id FROM data_sources WHERE slug = %s)",
                (source,),
            )
            conn.execute("DELETE FROM data_sources WHERE slug = %s", (source,))
            conn.execute("DELETE FROM companies WHERE domain = %s", (domain,))


@pytest.mark.skipif(
    not os.getenv("NEARHIVE_TEST_DATABASE_URL"),
    reason="set NEARHIVE_TEST_DATABASE_URL to a migrated disposable database",
)
def test_company_alias_and_domain_written() -> None:
    database_url = os.environ["NEARHIVE_TEST_DATABASE_URL"]
    suffix = uuid.uuid4().hex
    source = f"identity-test-{suffix}"
    domain = f"{suffix}.example"
    batch = EvidenceBatch(
        contract_version=1,
        discovery_job_id=uuid.uuid4(),
        source=source,
        source_family="official_site",
        observed_at=datetime.now(timezone.utc),
        companies=[
            CompanyEvidence(
                name="Identity Test Company",
                source_record_id=suffix,
                domain=domain,
                address="Test Road",
                lat=12.97,
                lng=77.59,
            )
        ],
        jobs=[],
    )

    try:
        with PostgresPersistence(database_url) as persistence:
            persistence.persist(batch)
            persistence.persist(batch)

        with psycopg.connect(database_url) as conn:
            company_id = conn.execute(
                "SELECT id FROM companies WHERE domain = %s", (domain,)
            ).fetchone()[0]
            aliases = conn.execute(
                """SELECT alias, normalized_alias, alias_type FROM company_aliases
                   WHERE company_id = %s AND alias_type = 'source_name'""",
                (company_id,),
            ).fetchall()
            domains = conn.execute(
                """SELECT normalized_domain, domain_type, is_primary, is_active
                   FROM company_domains WHERE company_id = %s""",
                (company_id,),
            ).fetchall()

        assert aliases == [("Identity Test Company", "identity test company", "source_name")]
        assert domains == [(domain, "primary", True, True)]
    finally:
        with psycopg.connect(database_url) as conn:
            conn.execute(
                "DELETE FROM source_records WHERE data_source_id IN (SELECT id FROM data_sources WHERE slug = %s)",
                (source,),
            )
            conn.execute(
                "DELETE FROM ingestion_runs WHERE data_source_id IN (SELECT id FROM data_sources WHERE slug = %s)",
                (source,),
            )
            conn.execute("DELETE FROM data_sources WHERE slug = %s", (source,))
            conn.execute("DELETE FROM sightings WHERE source = %s", (source,))
            conn.execute("DELETE FROM companies WHERE domain = %s", (domain,))


@pytest.mark.skipif(
    not os.getenv("NEARHIVE_TEST_DATABASE_URL"),
    reason="set NEARHIVE_TEST_DATABASE_URL to a migrated disposable database",
)
def test_location_lifecycle_fields_written() -> None:
    database_url = os.environ["NEARHIVE_TEST_DATABASE_URL"]
    suffix = uuid.uuid4().hex
    source = f"lifecycle-test-{suffix}"
    domain = f"{suffix}.example"
    batch = EvidenceBatch(
        contract_version=1,
        discovery_job_id=uuid.uuid4(),
        source=source,
        source_family="official_site",
        observed_at=datetime.now(timezone.utc),
        companies=[
            CompanyEvidence(
                name="Lifecycle Test Company",
                source_record_id=suffix,
                domain=domain,
                address="  12   Lifecycle Road  ",
                lat=12.97,
                lng=77.59,
            )
        ],
        jobs=[],
    )

    try:
        with PostgresPersistence(database_url) as persistence:
            persistence.persist(batch)
            with psycopg.connect(database_url) as conn:
                row = conn.execute(
                    """SELECT location_type, status, country_code, address_hash, first_seen_at, last_seen_at
                       FROM locations WHERE company_id = (SELECT id FROM companies WHERE domain = %s)""",
                    (domain,),
                ).fetchone()
                assert row is not None
                assert row[0] == "office"
                assert row[1] == "unverified"
                assert row[2] == "IN"
                assert row[3] is not None
                first_seen, last_seen = row[4], row[5]
            persistence.persist(batch)

        with psycopg.connect(database_url) as conn:
            locations = conn.execute(
                """SELECT id, address_hash, last_seen_at
                   FROM locations WHERE company_id = (SELECT id FROM companies WHERE domain = %s)""",
                (domain,),
            ).fetchall()

        assert len(locations) == 1
        assert locations[0][1] == row[3]
        assert locations[0][2] >= last_seen
        assert first_seen is not None
    finally:
        with psycopg.connect(database_url) as conn:
            conn.execute(
                "DELETE FROM source_records WHERE data_source_id IN (SELECT id FROM data_sources WHERE slug = %s)",
                (source,),
            )
            conn.execute(
                "DELETE FROM ingestion_runs WHERE data_source_id IN (SELECT id FROM data_sources WHERE slug = %s)",
                (source,),
            )
            conn.execute("DELETE FROM data_sources WHERE slug = %s", (source,))
            conn.execute("DELETE FROM sightings WHERE source = %s", (source,))
            conn.execute("DELETE FROM companies WHERE domain = %s", (domain,))


@pytest.mark.skipif(
    not os.getenv("NEARHIVE_TEST_DATABASE_URL"),
    reason="set NEARHIVE_TEST_DATABASE_URL to a migrated disposable database",
)
def test_job_canonical_fields_and_location_kind() -> None:
    database_url = os.environ["NEARHIVE_TEST_DATABASE_URL"]
    suffix = uuid.uuid4().hex
    source = f"canonical-test-{suffix}"
    domain = f"{suffix}.example"
    stated_batch = EvidenceBatch(
        contract_version=1,
        discovery_job_id=uuid.uuid4(),
        source=source,
        source_family="official_site",
        observed_at=datetime.now(timezone.utc),
        companies=[
            CompanyEvidence(
                name="Canonical Test Company",
                source_record_id=suffix,
                domain=domain,
                address="Test Road",
                lat=12.97,
                lng=77.59,
            )
        ],
        jobs=[
            TechnicalJobEvidence(
                company_name="Canonical Test Company",
                company_domain=domain,
                title="Software Engineer",
                source_job_id=suffix,
                canonical_url=f"https://{domain}/jobs/{suffix}",
                location_raw="Whitefield, Bengaluru",
                lat=12.98,
                lng=77.60,
                posted_at_confidence=0.75,
                technical_classification="software",
            )
        ],
    )
    coords_only_batch = replace(
        stated_batch,
        jobs=[
            replace(
                stated_batch.jobs[0],
                source_job_id=f"coords-{suffix}",
                canonical_url=f"https://{domain}/jobs/coords-{suffix}",
                location_raw="",
            )
        ],
    )

    try:
        with PostgresPersistence(database_url) as persistence:
            persistence.persist(stated_batch)
            persistence.persist(coords_only_batch)

        with psycopg.connect(database_url) as conn:
            job_row = conn.execute(
                """SELECT j.state, j.seniority, j.employment_type, j.activity_confidence,
                          r.slug
                   FROM technical_job_postings j
                   LEFT JOIN role_families r ON r.id = j.role_family_id
                   WHERE j.source = %s AND j.source_job_id = %s""",
                (source, suffix),
            ).fetchone()
            stated_location = conn.execute(
                """SELECT jl.location_kind, jl.country_code FROM job_locations jl
                   JOIN technical_job_postings j ON j.id = jl.job_id
                   WHERE j.source = %s AND j.source_job_id = %s""",
                (source, suffix),
            ).fetchone()
            inferred_location = conn.execute(
                """SELECT jl.location_kind FROM job_locations jl
                   JOIN technical_job_postings j ON j.id = jl.job_id
                   WHERE j.source = %s AND j.source_job_id = %s""",
                (source, f"coords-{suffix}"),
            ).fetchone()

        assert job_row is not None
        assert (job_row[0], job_row[1], job_row[2], job_row[3], job_row[4]) == (
            "open",
            "unknown",
            "unknown",
            0.75,
            "engineering",
        )
        assert stated_location == ("stated_job_location", "IN")
        assert inferred_location is not None and inferred_location[0] == "inferred"
    finally:
        with psycopg.connect(database_url) as conn:
            conn.execute(
                "DELETE FROM source_records WHERE data_source_id IN (SELECT id FROM data_sources WHERE slug = %s)",
                (source,),
            )
            conn.execute(
                "DELETE FROM ingestion_runs WHERE data_source_id IN (SELECT id FROM data_sources WHERE slug = %s)",
                (source,),
            )
            conn.execute("DELETE FROM data_sources WHERE slug = %s", (source,))
            conn.execute("DELETE FROM companies WHERE domain = %s", (domain,))


@pytest.mark.skipif(
    not os.getenv("NEARHIVE_TEST_DATABASE_URL"),
    reason="set NEARHIVE_TEST_DATABASE_URL to a migrated disposable database",
)
def test_inferred_does_not_overwrite_provider() -> None:
    database_url = os.environ["NEARHIVE_TEST_DATABASE_URL"]
    suffix = uuid.uuid4().hex
    source = f"coord-conflict-test-{suffix}"
    domain = f"{suffix}.example"
    batch = EvidenceBatch(
        contract_version=1,
        discovery_job_id=uuid.uuid4(),
        source=source,
        source_family="official_site",
        observed_at=datetime.now(timezone.utc),
        companies=[
            CompanyEvidence(
                name="Coord Conflict Test Company",
                source_record_id=suffix,
                domain=domain,
                address="Test Road",
                lat=12.97,
                lng=77.59,
            )
        ],
        jobs=[
            TechnicalJobEvidence(
                company_name="Coord Conflict Test Company",
                company_domain=domain,
                title="Software Engineer",
                source_job_id=suffix,
                canonical_url=f"https://{domain}/jobs/{suffix}",
                location_raw="Whitefield, Bengaluru",
                lat=12.98,
                lng=77.60,
                technical_classification="software",
            )
        ],
    )

    try:
        with PostgresPersistence(database_url) as persistence:
            persistence.persist(batch)
            persistence.persist(replace(batch, jobs=[replace(batch.jobs[0], lat=None, lng=None)]))

        with psycopg.connect(database_url) as conn:
            location_row = conn.execute(
                """SELECT jl.latitude, jl.longitude, jl.coordinate_source FROM job_locations jl
                   JOIN technical_job_postings j ON j.id = jl.job_id
                   WHERE j.source = %s AND j.source_job_id = %s""",
                (source, suffix),
            ).fetchone()

        assert location_row == (12.98, 77.60, "provider")
    finally:
        with psycopg.connect(database_url) as conn:
            conn.execute(
                "DELETE FROM source_records WHERE data_source_id IN (SELECT id FROM data_sources WHERE slug = %s)",
                (source,),
            )
            conn.execute(
                "DELETE FROM ingestion_runs WHERE data_source_id IN (SELECT id FROM data_sources WHERE slug = %s)",
                (source,),
            )
            conn.execute("DELETE FROM data_sources WHERE slug = %s", (source,))
            conn.execute("DELETE FROM companies WHERE domain = %s", (domain,))


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
