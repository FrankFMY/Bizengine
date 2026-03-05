# Технологический стек

## Язык: Go 1.24+

Обоснование: статическая типизация, goroutines для concurrency (event bus, view invalidation), быстрая компиляция, single binary deployment, отличная stdlib для HTTP/JSON/crypto.

Не используем: generics для domain-типов (избыточная сложность). Используем generics только для утилит (PageResponse[T]).

## База данных: PostgreSQL 16

Обоснование: JSONB для гибких компонентов (ECS), pg_trgm для полнотекстового поиска, RLS для мультитенантности, NUMERIC для денег, отличная поддержка в Go через pgx.

Драйвер: `github.com/jackc/pgx/v5` — нативный PostgreSQL драйвер, не database/sql.
Pool: `pgxpool` — встроенный connection pool.

Настройки пула:
- MaxConns: 25
- MinConns: 5
- MaxConnLifetime: 1 hour
- MaxConnIdleTime: 30 minutes

## Кеш и сессии: Redis 7

Обоснование: хранение сессий (teco_session/teco_seance), кеш часто читаемых данных (каталог, остатки), pub/sub для масштабирования на несколько инстансов.

Драйвер: `github.com/redis/go-redis/v9`

Используется для:
- Session store (teco_session + teco_seance cookies)
- Кеш entity/components (TTL 5 min)
- Кеш stock levels (TTL 1 min)
- Pub/sub bridge при горизонтальном масштабировании

## Real-time: Centrifugo v6

Обоснование: выделенный real-time сервер для масштабируемой доставки обновлений. Заменяет in-process WebSocket Hub. Поддерживает connect/subscribe proxy для интеграции с авторизацией BizEngine.

Конфигурация: `deploy/centrifugo.json`

Используется для:
- Доставка view_snapshot, table_diff, view_diff сообщений клиентам
- Каналы: `workspace:<wsID>` (general), `views:<wsID>` (view updates)
- Connect proxy → `/api/internal/centrifugo/connect` (валидация cookies)
- Subscribe proxy → `/api/internal/centrifugo/subscribe` (валидация доступа к каналу)

## Очередь событий: NATS JetStream

Обоснование: event streaming с гарантией at-least-once доставки, легковесный, кластеризация. На MVP — in-process event bus. NATS подключается при горизонтальном масштабировании.

Драйвер: `github.com/nats-io/nats.go`

Стратегия: Event Bus имеет два режима:
1. `LocalBus` — in-process, goroutine fan-out (default, достаточно для одного инстанса)
2. `NATSBus` — distributed, NATS JetStream (включается переменной окружения)

Оба реализуют один интерфейс `EventPublisher`.

## HTTP Router: Chi v5

`github.com/go-chi/chi/v5`

Обоснование: совместим с net/http, middleware chain, URL params, groups. Минимальная зависимость.

## Аутентификация: Cookie Sessions

`golang.org/x/crypto/bcrypt` для хеширования паролей. Redis для хранения сессий.

Схема: session cookie `teco_session` (72h) + seance cookie `teco_seance` (10m auto-refresh). Обе cookies HttpOnly, SameSite=Lax. Данные сессии хранятся в Redis.

## Логирование: zerolog

`github.com/rs/zerolog`

Обоснование: структурированный JSON-лог, zero-allocation, быстрый. В dev — ConsoleWriter.

## Конфигурация: envconfig

`github.com/sethvargo/go-envconfig`

Все параметры через переменные окружения. Никаких config файлов. .env только для локальной разработки.

## UUID: google/uuid

`github.com/google/uuid` — v4 для всех ID.

## YAML: gopkg.in/yaml.v3

Для парсинга определений бизнес-процессов.

## Тестирование

- Стандартная библиотека `testing`
- `github.com/stretchr/testify` — assertions и mocks
- `github.com/testcontainers/testcontainers-go` — интеграционные тесты с реальным Postgres

## Полный go.mod (актуальный)

```
module github.com/bizengine/engine

go 1.24.0

require (
    github.com/go-chi/chi/v5 v5.2.5
    github.com/google/uuid v1.6.0
    github.com/jackc/pgx/v5 v5.8.0
    github.com/redis/go-redis/v9 v9.18.0
    github.com/rs/zerolog v1.34.0
    github.com/sethvargo/go-envconfig v1.1.0
    github.com/stretchr/testify v1.11.1
    github.com/testcontainers/testcontainers-go v0.40.0
    github.com/testcontainers/testcontainers-go/modules/postgres v0.40.0
    golang.org/x/crypto v0.48.0
    gopkg.in/yaml.v3 v3.0.1
)
```

## Что явно НЕ используется и почему

| Технология | Причина отказа |
|---|---|
| GORM / ent / sqlc | ORM создаёт абстракцию, которая мешает при JSONB-запросах и сложных JOIN. Чистый SQL через pgx даёт полный контроль. |
| gRPC (для API) | Фронтенд-клиент в браузере. REST + Centrifugo проще для интеграции. gRPC остаётся опцией для inter-service communication при будущем разделении на микросервисы. |
| GraphQL | Сложность сервера, N+1 проблемы с ECS-моделью. REST с include-параметрами решает задачу проще. |
| Kafka | Overkill для целевой нагрузки (МСБ). NATS JetStream покрывает нужды при меньшей операционной сложности. |
| MongoDB | Реляционные данные (заказы ↔ товары ↔ склады), транзакции, ACID. PostgreSQL + JSONB даёт гибкость документов с надёжностью реляционной БД. |
| JWT | Переход на cookie sessions: упрощает интеграцию с Centrifugo (cookies forwarded автоматически), устраняет сложность refresh token rotation, HttpOnly cookies безопаснее чем токены в localStorage. |
| gorilla/websocket | Заменён на Centrifugo — выделенный real-time сервер с горизонтальным масштабированием, presence, автоматическим reconnect. |
