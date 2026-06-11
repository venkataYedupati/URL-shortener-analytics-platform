# URL Shortener + Analytics Platform

High-throughput URL shortener with Redis-backed redirects, Kafka click events, PostgreSQL persistence, and a React analytics dashboard.

## Architecture

- Go API for link creation, redirect resolution, QR generation, rate limiting, and analytics reads.
- PostgreSQL for canonical links, tenants/domains, click rollups, and recent event samples.
- Redis for hot redirect lookups and token-bucket rate limiting.
- Kafka for durable click event ingestion from redirects to analytics aggregation.
- React + TypeScript dashboard for creating links and exploring geo, device, referrer, and time-series analytics.
- Docker Compose for local development and AWS ECS deployment scaffolding for production.

## Local Development

```bash
cp .env.example .env
docker compose up --build
```

Services:

- API: http://localhost:8080
- Dashboard: http://localhost:5173
- Kafka UI: http://localhost:8090
- Postgres: localhost:5432
- Redis: localhost:6379

Run tests locally:

```bash
go test ./...
npm --prefix web install
npm --prefix web run build
```

Run a local smoke test after Compose is healthy:

```bash
bash scripts/smoke.sh
```

## API Snapshot

- `POST /v1/links` creates a short URL.
- `GET /{code}` redirects and publishes a click event.
- `GET /v1/links/{code}` returns link metadata.
- `GET /v1/links/{code}/analytics` returns rollups and recent events.
- `GET /v1/links/{code}/qr` returns a PNG QR code for the short URL.
- `GET /healthz` returns dependency health.

Create a link:

```bash
curl -X POST http://localhost:8080/v1/links \
  -H "Content-Type: application/json" \
  -d '{"target_url":"https://example.com","title":"Example","custom_code":"demo"}'
```

Trigger a redirect:

```bash
curl -I http://localhost:8080/demo
```

Read analytics:

```bash
curl http://localhost:8080/v1/links/demo/analytics?hours=24
```

## Deployment

The `infra/aws` folder includes ECS task definition and autoscaling policy templates for API and worker services. In production, store secrets in SSM Parameter Store or Secrets Manager, run Postgres on RDS, Redis on ElastiCache, and Kafka on MSK or a Kafka-compatible managed service.

Build images from the repository root:

```bash
docker build --build-arg APP_TARGET=api -t url-shortener-api .
docker build --build-arg APP_TARGET=worker -t url-shortener-worker .
docker build -t url-shortener-web ./web
```

## Resume-Scale Signals

- Redirect path uses Redis cache with Postgres fallback and a publish-only Kafka click event path.
- Analytics worker consumes click events independently and upserts rollups by time bucket, country, device, and referrer.
- Token-bucket rate limiting protects write APIs and can be tuned with environment variables.
- Dockerized services mirror production concerns: API, worker, web, Postgres, Redis, and Kafka.
