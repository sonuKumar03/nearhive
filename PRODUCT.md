# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

NearHive primarily serves technical job seekers in India who want to discover companies near a chosen or current location and understand which of those companies are actively hiring. Users also need India-eligible remote opportunities and trustworthy company intelligence before deciding where to apply.

## Product Purpose

NearHive builds a source-traceable company and hiring dataset. Its primary outcome is a fast, reliable answer to: which companies are located nearby, which companies are hiring nearby or remotely, and what current jobs support that conclusion?

The product later exposes saved companies, reviews, compensation information, interview experiences, and interview questions. Those capabilities depend on the database and governance model defined first; the current scope does not redesign the frontend.

## Positioning

NearHive combines verified physical company presence, current job evidence, work-arrangement classification, and source provenance. It does not infer that every job is local merely because a company has a nearby office.

## Operating Context

- A user searches around current coordinates or a selected Indian city and radius.
- Results combine nearby in-office and hybrid hiring with India-eligible remote hiring in one candidate dataset.
- Company hiring classifications are derived multi-label facts, not manually assigned categories.
- Technical roles launch first; the canonical schema must support other role families later.
- Reviews, compensation records, and interview experiences may come from users, companies/admins, licensed imports, or public sources.

## Capabilities and Constraints

- India-first, globally extensible country, currency, location, and remote-eligibility fields.
- Initial production target: approximately 1 million companies, 10 million historical jobs, and 100,000 active users.
- PostgreSQL/PostGIS remains the canonical database and initial query engine.
- Canonical facts must retain source, observation time, confidence, and rule-version provenance.
- User contributions are publicly anonymous but linked internally to authenticated accounts for moderation and abuse control.
- Salary aggregates are not published until at least five distinct approved contributors exist in the same cohort.
- The current work concerns the dataset and database. A richer web experience or dedicated SPA is intentionally deferred.

## Evidence on Hand

- Existing PostgreSQL/PostGIS company, location, user, search-history, discovery-job, source-run, sighting, and technical-job tables.
- Existing Go canonical ingestion and nearby-company search.
- Existing Python discovery adapters for OSM, configured directories, company sites, JSON-LD, Greenhouse, and Lever.
- Existing deterministic technical-role and work-arrangement classification.
- No existing saved-company, review, compensation, interview-experience, interview-question, moderation, or reputation datasets.

## Product Principles

- Evidence before claims: every location, hiring label, salary aggregate, and imported fact remains source-traceable.
- Local means local: nearby hiring requires job or office evidence appropriate to the work arrangement.
- Freshness is explicit: observed, posted, active, closed, and stale are different states.
- Privacy by aggregation: protect contributors while preserving internal accountability.
- Start with PostgreSQL: add secondary infrastructure only after measured query or ingestion limits.

## Accessibility & Inclusion

Future user-facing work must support keyboard use, assistive technology, and clear non-color status labels. Dataset terminology must distinguish remote eligibility from physical location and avoid presenting low-confidence information as verified fact.
