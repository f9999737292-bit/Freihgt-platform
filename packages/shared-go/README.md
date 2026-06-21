# shared-go

Shared Go libraries for Freight Platform backend services.

## Packages

| Package | Description |
|---------|-------------|
| `config` | Environment-based configuration loading |
| `logger` | JSON structured logging via `slog` |
| `health` | Health check HTTP handlers |
| `server` | HTTP server with graceful shutdown |

## Usage

```go
import (
    "github.com/freight-platform/shared-go/config"
    "github.com/freight-platform/shared-go/logger"
    "github.com/freight-platform/shared-go/server"
)

cfg, _ := config.Load("my-service", 8080)
log := logger.New(cfg.ServiceName, cfg.LogLevel, cfg.Environment)
srv := server.New(cfg.ServiceName, cfg.HTTPPort, log)
```

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `HTTP_PORT` | service default | HTTP listen port |
| `ENVIRONMENT` | `development` | Runtime environment |
| `LOG_LEVEL` | `info` | Log level (`debug`, `info`, `warn`, `error`) |
| `DB_MAX_OPEN_CONNS` | `25` | Max PostgreSQL pool connections |
| `DB_MAX_IDLE_CONNS` | `10` | Min idle pool connections |
| `DB_CONN_MAX_LIFETIME_SECONDS` | `300` | Connection max lifetime |
| `DB_CONN_MAX_IDLE_TIME_SECONDS` | `60` | Idle connection max lifetime |
| `DB_SLOW_QUERY_THRESHOLD_MS` | `500` | Slow query warn log threshold (`0` = off) |
