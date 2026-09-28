# NearHive Design & Planning Docs

Status of every design spec and implementation plan in this directory. The
**authoritative** architecture is the Python-Owned Discovery and Go Read API plan:
Python owns discovery and all canonical scraped-data writes; Go serves
authentication and read-only company/job queries.

| Document | Date | Status |
| --- | --- | --- |
| [plans/2026-09-28-python-owned-discovery-architecture.md](plans/2026-09-28-python-owned-discovery-architecture.md) | 2026-09-28 | ✅ **Current / authoritative** |
| [plans/2026-09-26-react-frontend-implementation.md](plans/2026-09-26-react-frontend-implementation.md) | 2026-09-26 | ✅ Implemented — Next.js frontend |
| [specs/2026-09-26-react-frontend-design.md](specs/2026-09-26-react-frontend-design.md) | 2026-09-26 | ✅ Current — frontend design |
| [specs/2026-09-28-company-intelligence-database-design.md](specs/2026-09-28-company-intelligence-database-design.md) | 2026-09-28 | 🟡 Draft / forward-looking — reconcile ownership before planning |
| [plans/2026-09-28-location-based-job-discovery-upgrade.md](plans/2026-09-28-location-based-job-discovery-upgrade.md) | 2026-09-28 | 🟠 Partially superseded — job-location rules retained; Go ingestion steps superseded |
| [plans/2026-09-27-python-company-discovery.md](plans/2026-09-27-python-company-discovery.md) | 2026-09-27 | ⛔ Superseded — Python sent evidence through Go |
| [specs/2026-09-27-python-company-discovery-design.md](specs/2026-09-27-python-company-discovery-design.md) | 2026-09-27 | ⛔ Superseded — Go-mediated evidence bridge |
| [plans/2026-09-26-standalone-crawler-service.md](plans/2026-09-26-standalone-crawler-service.md) | 2026-09-26 | ⛔ Superseded — Go crawler service retired |
| [plans/2026-09-26-stateless-concurrent-crawler.md](plans/2026-09-26-stateless-concurrent-crawler.md) | 2026-09-26 | ⛔ Superseded — Go crawler daemon retired |
| [specs/2026-09-26-standalone-crawler-service-infra-spec.md](specs/2026-09-26-standalone-crawler-service-infra-spec.md) | 2026-09-26 | ⛔ Superseded — crawler microservice infra retired |
| [plans/2026-09-25-nearhive-implementation-plan.md](plans/2026-09-25-nearhive-implementation-plan.md) | 2026-09-25 | ⛔ Superseded — Go scraper + REST API plan |
| [specs/2026-09-25-nearhive-design.md](specs/2026-09-25-nearhive-design.md) | 2026-09-25 | ⛔ Superseded as implementation guidance — full-stack Go design |
| [../ARCHITECTURE_AUDIT.md](../ARCHITECTURE_AUDIT.md) | 2026-09-26 | 🗓️ Historical — point-in-time audit, findings largely resolved |

> Superseded documents are kept for history and carry a status banner at the top.
> They must not be executed as written.
