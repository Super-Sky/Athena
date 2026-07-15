# Athena Local Docker Runtime

## Purpose

`deploy/docker-compose.runtime.yml` is Athena's local business-app runtime profile. It starts the API, PostgreSQL, and Redis, and can start the Web Control Plane with the `control-plane` profile. It contains no fund, holding, trade, or business-evidence tables.

## Start

```bash
cp deploy/athena.runtime.env.example deploy/athena.runtime.env
docker compose --env-file deploy/athena.runtime.env -f deploy/docker-compose.runtime.yml up --build -d
curl -fsS http://127.0.0.1:8080/healthz
```

To include the Control Plane UI:

```bash
COMPOSE_PROFILES=control-plane \
docker compose --env-file deploy/athena.runtime.env -f deploy/docker-compose.runtime.yml up --build -d
```

## Services and Healthchecks

- `postgres`: `pg_isready`.
- `redis`: `redis-cli ping`.
- `athena-api`: runs `/app/athena migrate`, starts `api-server`, and probes it through `/app/athena healthcheck`.
- `athena-web`: optional profile, checks the Nginx root path.

The API starts only after PostgreSQL and Redis are healthy. A migration failure is not hidden as a healthy service.

## Fund Assistant Integration

A fund assistant on the same Compose network must use:

```dotenv
ATHENA_BASE_URL=http://athena-api:8080
ATHENA_AUTH_TOKEN=
```

For a host-run fund assistant, use `ATHENA_EXTERNAL_BASE_URL` (default `http://127.0.0.1:8080`). Set `ATHENA_AUTH_TOKEN` only when Athena explicitly enables authentication middleware; never commit the token.

Redis is included for future caching, rate limiting, and asynchronous work. Athena core does not use Redis as a business-truth database.
