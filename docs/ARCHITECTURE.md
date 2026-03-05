# Архитектура BizEngine

## 1. Архитектурный паттерн: ECS + Event Sourcing + Modular Monolith

### Entity-Component-System (ECS)

Заимствование из игровых движков. Вместо жёстких таблиц для каждого типа объекта — универсальная модель:

- **Entity** — идентификатор + kind + name + status. Это «паспорт» объекта. Товар, склад, заказ, сотрудник, машина, точка продажи — всё Entity.
- **Component** — пакет данных, прикреплённый к Entity. Один Entity может иметь N компонентов разных типов. Компонент — это JSONB с типизированной схемой.
- **System (Module)** — бизнес-логика, оперирующая сущностями и компонентами. Warehouse System работает с entities kind=product + component type=inventory. Order System работает с entities kind=order + components price, inventory.

Почему: у малого бизнеса кофейня может иметь 5 типов сущностей, у логистической компании — 20. Жёсткая схема не масштабируется по бизнес-доменам. ECS позволяет добавлять новые типы без миграций.

### Event Sourcing

Каждое изменение состояния — событие в append-only логе:
- Аудит из коробки (кто, когда, что изменил).
- Real-time визуализация: фронтенд подписывается на reactive views через Centrifugo.
- Replay: можно восстановить состояние на любой момент времени.
- Reactive: модули реагируют на события друг друга без прямых зависимостей.

### Modular Monolith

Не микросервисы. Один бинарник, но строгие границы между модулями:
- Модули не импортируют друг друга.
- Взаимодействие только через Event Bus.
- Каждый модуль имеет свой Repository interface и свои миграции.
- При необходимости — модуль можно вынести в отдельный сервис, заменив in-process events на NATS.

## 2. Слои системы

```
┌─────────────────────────────────────────────────┐
│                   API Layer                      │
│                REST (Chi) + Views API            │
├─────────────────────────────────────────────────┤
│                 Auth / RBAC                      │
│     Cookie Sessions + Permissions + Workspace    │
├─────────────────────────────────────────────────┤
│              Module Layer (Systems)              │
│  Catalog │ Warehouse │ Order │ HR │ Finance │ …  │
├─────────────────────────────────────────────────┤
│                  Core Layer                      │
│      Entity │ Event Bus │ Process Engine         │
├────────────────────┬────────────────────────────┤
│  Reactive Views    │       Storage Layer         │
│  Engine + Triggers │  PostgreSQL │ Redis │ NATS  │
├────────────────────┴────────────────────────────┤
│               Real-time Layer                    │
│   Centrifugo v6 (connect/subscribe proxy)        │
└─────────────────────────────────────────────────┘
```

Направление зависимостей: API → Module → Core → Storage. Никогда наоборот.

## 3. Мультитенантность

Модель: **shared database, shared schema, workspace_id filter**.

- Каждый клиент = workspace.
- Каждая таблица с бизнес-данными имеет колонку `workspace_id`.
- Каждый SQL запрос обязательно фильтрует по `workspace_id`.
- Row-Level Security в PostgreSQL как дополнительный слой защиты.
- В контексте HTTP-запроса workspace_id извлекается из cookie session (Redis) и помещается в context.Context.

Workspace isolation проверяется на трёх уровнях:
1. API middleware: извлекает workspace_id из session cookies (Redis lookup).
2. Service layer: все методы принимают workspace_id первым аргументом.
3. Database: RLS policies (defense in depth).

## 4. Event Flow

```
[Действие пользователя / API / Таймер / Webhook]
        │
        ▼
[REST Handler] → валидация input → извлечение workspace_id из session
        │
        ▼
[Module Service] → бизнес-логика → валидация правил
        │
        ▼
[Entity Service] → мутация entity + components (PostgreSQL transaction)
        │
        ▼
[Event Store] → INSERT event (в той же транзакции)
        │
        ▼
[Event Bus] → fan-out после COMMIT
        │
        ├──▶ [Process Engine] → проверка переходов state machine
        ├──▶ [Views Engine] → invalidate views → Centrifugo push
        ├──▶ [Other Modules] → реакция (напр. warehouse слушает order.confirmed)
        └──▶ [Integration] → внешние системы (ФНС, ЭДО — async)
```

Критически важно: Event Store INSERT и мутация entity — в одной PostgreSQL транзакции. Event Bus fan-out — ПОСЛЕ коммита транзакции (outbox pattern не нужен при in-process bus, нужен при NATS).

## 5. Модули и их ответственности

### Core

| Модуль | Ответственность |
|---|---|
| `core/entity` | CRUD для entities + components. Optimistic concurrency через version. |
| `core/event` | Event Bus (publish/subscribe), Event Store (persist/query). |
| `core/process` | Загрузка YAML-определений. Исполнение state machines. Автоматические переходы по событиям. |
| `core/auth` | Cookie sessions (teco_session/teco_seance + Redis). RBAC (role → permissions). Middleware. |

### Modules

| Модуль | Entities | Components | Ключевые операции |
|---|---|---|---|
| `catalog` | product, category, service | price, media, attributes | CRUD товаров, категорий, SKU |
| `warehouse` | warehouse, zone | inventory, geo | Приход, расход, перемещение, инвентаризация, мин. остатки |
| `order` | order, order_item | price, delivery, payment | Создание, подтверждение, оплата, сборка, отгрузка |
| `hr` | employee, department, shift | contact, schedule, salary | Штатное расписание, смены, начисления |
| `finance` | account, transaction, invoice | financial | Двойная запись, счета, платежи, отчёты |
| `logistics` | vehicle, route, delivery | geo, schedule | Трекинг, маршруты, статусы доставок |

### Integrations (Phase 3-4)

| Интеграция | Назначение |
|---|---|
| `fns` | Онлайн-кассы (ФЗ-54), чеки |
| `edo` | Электронный документооборот (Диадок, СБИС, Контур) |
| `chestnyznak` | Маркировка товаров |
| `bank` | Выгрузка/загрузка банковских выписок (1С формат) |

## 6. Взаимодействие между модулями

Модули НЕ импортируют друг друга. Вся коммуникация — через события.

Пример: заказ подтверждён → склад должен зарезервировать товар.

```
[Order Module] → publishes event: order.confirmed { order_id, items: [{product_id, qty}] }
       │
       ▼
[Event Bus] → delivers to Warehouse Module subscriber
       │
       ▼
[Warehouse Module] → обрабатывает: резервирует stock → publishes: warehouse.stock.reserved
       │
       ▼
[Process Engine] → order_fulfillment state machine: confirmed → paid → assembling
```

Если нужна синхронная проверка между модулями (напр. проверить наличие товара перед созданием заказа), используется **Query Interface** — read-only интерфейс, зарегистрированный в DI:

```go
// В order service
type StockChecker interface {
    CheckAvailability(ctx context.Context, wsID uuid.UUID, items []OrderItem) error
}
```

Warehouse модуль реализует этот интерфейс. Инъекция через cmd/server/main.go. Модули по-прежнему не импортируют друг друга — интерфейс определён в потребителе.

## 7. Reactive Views Engine

Система реактивных представлений, обеспечивающая real-time UI обновления через Centrifugo.

### Архитектура

```
[Client subscribes to view] → POST /views/subscribe
        │
        ▼
[ViewManager] → factory builds view → snapshot → Centrifugo publish
        │
[Event Bus event fires]
        │
        ▼
[Triggers] → определяет какие views затронуты событием
        │
        ▼
[ViewManager.Invalidate] → re-run factory → compute diff → Centrifugo publish
        │
        ▼
[Centrifugo] → delivers view_diff / table_diff to subscribed clients
```

### Ключевые концепции

- **View Definition**: зарегистрированное представление с key, params schema, factory function, triggers.
- **Factory**: функция, строящая view snapshot (refs + tables) по params.
- **Triggers**: mapping event_type → view keys, определяющий какие views инвалидируются при каком событии.
- **Refs**: нормализованные ссылки на данные `[{table, id, fields}]` — вместо дублирования данных.
- **Tables**: денормализованные данные `{table: {id: {fields}}}` — фактические данные для отображения.

### Сообщения Centrifugo

| Message Type | Когда | Содержимое |
|---|---|---|
| `view_snapshot` | Первичная подписка | Полный snapshot: refs + tables + version |
| `view_diff` | Изменение состава view | refs_patch (JSON Patch) + новые tables |
| `table_diff` | Изменение существующей записи | table + id + patch (JSON Patch) |

### SDK

TypeScript SDK в `sdk/`:
- `views.d.ts` — автогенерируемые типы (20 views, params + result) через `scripts/gen_view_types.go`
- `client.ts` — клиентский скелет ArcanaClient для Svelte 5 интеграции

## 8. Конфигурация

Все параметры — через переменные окружения. Prefixes:

```
SERVER_HOST, SERVER_PORT
DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME, DB_SSLMODE
REDIS_ADDR, REDIS_PASSWORD, REDIS_DB
NATS_URL, NATS_ENABLED (true/false, default false)
SESSION_TTL (default "72h"), SESSION_SEANCE_TTL (default "10m"), SESSION_COOKIE_SECURE (default false)
CENTRIFUGO_API_URL (default "http://localhost:8001/api"), CENTRIFUGO_API_KEY
LOG_LEVEL (debug, info, warn, error)
ENV (development, staging, production)
```

## 9. Graceful Shutdown

Порядок:
1. Прекратить приём новых HTTP-соединений.
2. Дождаться завершения активных HTTP-запросов (timeout 30s).
3. Дождаться завершения event handlers (timeout 10s).
4. Закрыть NATS (если используется).
5. Закрыть Redis.
6. Закрыть PostgreSQL pool.

Centrifugo — отдельный процесс, управляется через docker-compose.

## 10. Observability (заложить, реализовать в Phase 2+)

- Structured JSON logs (zerolog) — с request_id, workspace_id, user_id.
- Prometheus metrics endpoint `/metrics` (http request duration, event bus throughput, pg pool stats).
- Health check `/health` — проверяет Postgres, Redis.
- Ready check `/ready` — то же + NATS (если включён).
