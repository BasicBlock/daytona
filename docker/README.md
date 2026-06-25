# Docker Compose Setup for Daytona

This folder contains a Docker Compose setup for running Daytona locally.

Important:

- This setup is still in development and is not safe to use in production.
- A separate deployment guide should be used for production scenarios.

## Overview

The Docker Compose configuration includes:

- API: main Daytona application server
- Proxy: sandbox preview and toolbox proxy
- Runner: service that hosts sandboxes
- SSH Gateway: sandbox SSH access
- PostgreSQL: data persistence
- Redis: cache and coordination
- Registry: Docker image registry with web UI
- MinIO: S3-compatible object storage
- Jaeger: distributed tracing
- PgAdmin: database administration UI

## Quick Start

Start all services from the repo root:

```bash
docker compose -f docker/docker-compose.yaml up -d
```

Useful local URLs:

- Daytona Dashboard: http://localhost:3000
- PgAdmin: http://localhost:5050
- Registry UI: http://localhost:5100
- MinIO Console: http://localhost:9001

## Proxy DNS

For local preview URLs, resolve `*.proxy.localhost` to `127.0.0.1`:

```bash
./scripts/setup-proxy-dns.sh
```

Without this setup, SDK examples and direct proxy access may not resolve.

## Development Notes

- Database, registry, and object-storage data are persisted in Docker volumes.
- The registry allows image deletion for local testing.
- Sandbox resource limits are disabled in this local setup because Docker-in-Docker cannot reliably partition cgroups without the host socket.
