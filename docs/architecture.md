# Architecture

## Request Path

1. `POST /v1/links` validates the destination URL, applies Redis token-bucket rate limiting, writes the canonical link row to Postgres, and warms Redis with the redirect metadata.
2. `GET /{code}` checks Redis first. On a miss, it reads Postgres, rehydrates Redis, publishes a compact Kafka click event, and redirects with `302 Found`.
3. The analytics worker consumes click events, stores recent raw events, increments the link total, and upserts hourly rollups for country, device, and referrer dimensions.
4. The dashboard reads `/v1/overview`, `/v1/links/{code}/analytics`, and `/v1/links/{code}/qr`.

## Scaling Notes

- API tasks are horizontally stateless. Redis carries the hot redirect cache and rate-limit buckets.
- Kafka decouples redirect latency from analytics writes. Redirects can continue even if the worker falls behind.
- Postgres stores canonical state and aggregated analytics; high-volume deployments can partition `click_events` by month and retain rollups longer than raw events.
- Custom domains resolve through the same code path. The API normalizes request hosts and checks domain-specific links before generic links.
- QR generation is stateless and derived from the configured public short URL.

## Operational Defaults

- Redirect cache TTL is 10 minutes or shorter if the link expires sooner.
- Write API rate limits default to 120 requests per IP hash per 60 seconds.
- Analytics queries default to 24 hours and cap at 90 days.
- Kafka topic auto-creation is enabled in local Compose only; production should pre-create topics with explicit retention and partition counts.
- Click ingestion is idempotent by `request_id`, so a worker retry after a Kafka offset commit failure does not double-count the click.
- Link creation rejects localhost and private-network target URLs to reduce SSRF risk.
- JSON request bodies are capped and decoded strictly to avoid oversized or ambiguous payloads.
