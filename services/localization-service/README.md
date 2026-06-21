# localization-service

Localization and translations for Freight Platform.

## Run locally

```bash
cp .env.example .env
go run ./cmd/server
```

## Health check

```bash
curl http://localhost:8083/health
```

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `HTTP_PORT` | `8083` | HTTP listen port |
| `ENVIRONMENT` | `development` | Runtime environment |
| `LOG_LEVEL` | `info` | Log level |

## Docker

Build from monorepo root:

```bash
docker build -f services/localization-service/Dockerfile -t freight-platform/localization-service .
```
