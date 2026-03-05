# Deployment

## Development

```bash
# Поднять инфраструктуру
docker compose up -d postgres redis centrifugo

# Применить миграции
make migrate-up

# Запустить с hot reload
make dev
```

docker-compose.yml содержит: PostgreSQL 16 (port 5436), Redis 7 (port 6381), NATS 2.10 + JetStream (port 4222), Centrifugo v6 (port 8001).

## Production

### Docker

Multi-stage build: Go builder → Alpine runtime. Single binary ~15MB.

```dockerfile
FROM golang:1.24-alpine AS builder
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/engine ./cmd/server

FROM alpine:3.20
COPY --from=builder /bin/engine /bin/engine
COPY migrations /app/migrations
COPY processes /app/processes
ENTRYPOINT ["/bin/engine"]
```

### Environment

Все настройки через ENV. Никаких конфиг-файлов в контейнере. Centrifugo настраивается через `deploy/centrifugo.json`.

### Health checks

- `/health` — liveness probe (Postgres + Redis ping)
- `/ready` — readiness probe (то же + проверка миграций)

### Масштабирование

1 инстанс: достаточно для <100 concurrent users (типичный МСБ).
N инстансов: включить NATS_ENABLED=true, Centrifugo уже stateless (масштабируется независимо).

### Минимальные ресурсы

- Engine: 256MB RAM, 0.5 CPU
- PostgreSQL: 1GB RAM, 1 CPU
- Redis: 128MB RAM
- NATS: 64MB RAM
- Centrifugo: 128MB RAM, 0.5 CPU

### Бэкапы

PostgreSQL: pg_dump ежедневно. WAL archiving для point-in-time recovery.

## CI/CD (рекомендация)

```yaml
# GitHub Actions
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:16-alpine
        env: { POSTGRES_USER: test, POSTGRES_PASSWORD: test, POSTGRES_DB: test }
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.24' }
      - run: go vet ./...
      - run: go test ./... -race -count=1
  build:
    needs: test
    steps:
      - run: docker build -f deploy/Dockerfile -t bizengine .
```
