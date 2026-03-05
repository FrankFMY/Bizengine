# Event System

## 1. Принципы

- Каждая мутация данных в системе порождает событие.
- Событие сохраняется в Event Store (PostgreSQL, append-only) в той же транзакции, что и мутация.
- После коммита транзакции событие рассылается подписчикам через Event Bus.
- Подписчики обрабатывают события асинхронно (goroutine pool).
- Событие — факт, который уже произошёл. Именование: прошедшее время (`order.created`, не `order.create`).

## 2. Структура события

```go
type Event struct {
    ID          uuid.UUID       `json:"id"`
    WorkspaceID uuid.UUID       `json:"workspace_id"`
    EntityID    *uuid.UUID      `json:"entity_id,omitempty"`
    Type        string          `json:"type"`
    Data        json.RawMessage `json:"data"`
    ActorID     *uuid.UUID      `json:"actor_id,omitempty"`
    Timestamp   time.Time       `json:"timestamp"`
    Version     int64           `json:"version"`
}
```

## 3. Каталог событий

### Core

| Event Type | Entity Kind | Data Schema | Trigger |
|---|---|---|---|
| `entity.created` | any | `{id, kind, name, status}` | Создание сущности |
| `entity.updated` | any | `{id, changes: {field: {old, new}}}` | Обновление сущности |
| `entity.deleted` | any | `{id, kind}` | Soft delete сущности |
| `component.set` | any | `{entity_id, type, data, version}` | Установка компонента |
| `component.removed` | any | `{entity_id, type}` | Удаление компонента |

### Catalog

| Event Type | Data Schema |
|---|---|
| `catalog.product.created` | `{id, name, sku, category_id}` |
| `catalog.product.updated` | `{id, changes}` |
| `catalog.product.archived` | `{id}` |
| `catalog.category.created` | `{id, name, parent_id}` |

### Warehouse

| Event Type | Data Schema |
|---|---|
| `warehouse.stock.received` | `{movement_id, product_id, warehouse_id, quantity, unit, cost_per_unit, reason}` |
| `warehouse.stock.shipped` | `{movement_id, product_id, warehouse_id, quantity, unit, reference_type, reference_id}` |
| `warehouse.stock.transferred` | `{movement_id, product_id, from_warehouse_id, to_warehouse_id, quantity, unit}` |
| `warehouse.stock.adjusted` | `{movement_id, product_id, warehouse_id, old_quantity, new_quantity, reason}` |
| `warehouse.stock.reserved` | `{product_id, warehouse_id, quantity, order_id}` |
| `warehouse.stock.unreserved` | `{product_id, warehouse_id, quantity, order_id}` |
| `warehouse.stock.low` | `{product_id, warehouse_id, quantity, min_quantity}` |

### Order

| Event Type | Data Schema |
|---|---|
| `order.created` | `{id, number, customer_id, items: [{product_id, quantity, unit_price}], total}` |
| `order.confirmed` | `{id, number}` |
| `order.paid` | `{id, number, amount, payment_method}` |
| `order.assembling` | `{id, number, warehouse_id, assigned_to}` |
| `order.shipped` | `{id, number, tracking}` |
| `order.delivered` | `{id, number}` |
| `order.cancelled` | `{id, number, reason}` |
| `order.refunded` | `{id, number, amount}` |

### HR

| Event Type | Data Schema |
|---|---|
| `hr.employee.hired` | `{id, name, position, department_id, hire_date}` |
| `hr.employee.terminated` | `{id, reason, effective_date}` |
| `hr.shift.started` | `{shift_id, employee_id, location_id}` |
| `hr.shift.completed` | `{shift_id, employee_id, hours_worked}` |
| `hr.timesheet.approved` | `{timesheet_id, employee_id, period}` |

### Finance

| Event Type | Data Schema |
|---|---|
| `finance.transaction.posted` | `{id, date, lines: [{account_id, debit, credit}]}` |
| `finance.invoice.created` | `{id, number, type, counterparty_id, total}` |
| `finance.invoice.paid` | `{id, number, amount, paid_at}` |
| `finance.payment.received` | `{order_id, amount, method}` |

### Logistics

| Event Type | Data Schema |
|---|---|
| `logistics.route.started` | `{route_id, vehicle_id, driver_id}` |
| `logistics.route.completed` | `{route_id}` |
| `logistics.stop.arrived` | `{stop_id, route_id, arrived_at}` |
| `logistics.stop.completed` | `{stop_id, route_id, delivery_ids}` |
| `logistics.geo.updated` | `{entity_id, latitude, longitude, speed, heading}` |

### Process

| Event Type | Data Schema |
|---|---|
| `process.started` | `{process_id, definition_id, entity_id, init_state}` |
| `process.transition` | `{process_id, from, to, trigger_event}` |
| `process.completed` | `{process_id, entity_id, final_state}` |
| `process.failed` | `{process_id, entity_id, error, state}` |

### System

| Event Type | Data Schema |
|---|---|
| `system.user.registered` | `{user_id, email}` |
| `system.workspace.created` | `{workspace_id, name, owner_id}` |
| `system.member.added` | `{workspace_id, user_id, role}` |
| `system.member.removed` | `{workspace_id, user_id}` |

## 4. Event Bus Interface

```go
type EventPublisher interface {
    Publish(ctx context.Context, event Event) error
}

type EventSubscriber interface {
    HandleEvent(ctx context.Context, event Event) error
}

type EventBus interface {
    EventPublisher
    Subscribe(eventType string, handler EventSubscriber)
    SubscribeAll(handler EventSubscriber)
}
```

## 5. Event Store Interface

```go
type EventStore interface {
    Append(ctx context.Context, event Event) error
    GetByEntity(ctx context.Context, wsID, entityID uuid.UUID, since *time.Time, limit int) ([]Event, error)
    GetByWorkspace(ctx context.Context, wsID uuid.UUID, limit, offset int) ([]Event, error)
    GetByType(ctx context.Context, wsID uuid.UUID, eventType string, since *time.Time, limit int) ([]Event, error)
}
```

## 6. Подписки (кто слушает что)

| Subscriber | Events | Action |
|---|---|---|
| Views Engine | по triggers | Invalidate views → compute diff → publish через Centrifugo |
| Process Engine | * (все) | Проверка переходов state machine |
| Warehouse Module | `order.confirmed` | Резервирование stock |
| Warehouse Module | `order.cancelled` | Снятие резерва |
| Finance Module | `order.paid` | Создание проводки (Дт 51 Кт 62) |
| Finance Module | `warehouse.stock.received` | Проводка поступления (Дт 41 Кт 60) |
| Logistics Module | `order.shipped` | Создание delivery задачи |

## 7. Гарантии доставки

### In-process (MVP, default)
- At-most-once: если handler паникует, событие теряется для этого handler.
- Event Store гарантирует persist: событие всегда сохранено в БД.
- Для критических handler'ов — retry с exponential backoff (3 попытки).

### NATS JetStream (horizontal scaling)
- At-least-once: JetStream гарантирует доставку с ack.
- Handler должен быть идемпотентным (проверять event.ID для дедупликации).

## 8. Outbox Pattern (при переходе на NATS)

При горизонтальном масштабировании — outbox table:
1. Event записывается в `events` в той же транзакции.
2. Фоновый worker читает непрочитанные events и публикует в NATS.
3. После подтверждения NATS — помечает event как published.

Не реализуем сейчас. Заложено в архитектуру через interface.
