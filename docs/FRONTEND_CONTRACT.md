# Контракт: Бэкенд ↔ Фронтенд

Документ для CTO (фронтенд). Описывает интерфейс между сервером и клиентом.

## Протоколы

| Канал | Протокол | Назначение |
|---|---|---|
| Операции с данными | REST JSON | CRUD, бизнес-операции |
| Real-time обновления | Centrifugo (WebSocket) | Reactive views — автоматическое обновление UI |
| Авторизация | Cookie sessions | HttpOnly cookies (teco_session + teco_seance) |

## SDK

Готовые файлы для интеграции в `sdk/`:
- `views.d.ts` — TypeScript типы для всех 21 views (автогенерация)
- `client.ts` — ArcanaClient с typed API для подписок и обработки сообщений

## Как фронтенд строит визуализацию «цифрового двойника»

### Шаг 1: Аутентификация

```js
// Login — cookies устанавливаются автоматически
await fetch('/api/v1/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify({ email, password })
});

// Switch workspace
await fetch('/api/v1/auth/switch', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify({ workspace_id: wsID })
});
```

### Шаг 2: Подключение к Centrifugo

```js
import { Centrifuge } from 'centrifuge';

const centrifuge = new Centrifuge('ws://host:8001/connection/websocket');
centrifuge.connect();

// Подписка на канал views
const sub = centrifuge.newSubscription(`views:${wsID}`);
sub.on('publication', (ctx) => {
    const msg = ctx.data;
    switch (msg.type) {
        case 'view_snapshot': client.handleSnapshot(msg); break;
        case 'table_diff':    client.handleTableDiff(msg); break;
        case 'view_diff':     client.handleViewDiff(msg); break;
    }
});
sub.subscribe();
```

### Шаг 3: Подписка на views (REST)

```js
import { ArcanaClient } from './sdk/client';

const client = new ArcanaClient('/api/v1');
client.setWorkspace(wsID);

// Подписаться на view — получить начальный snapshot через Centrifugo
const result = await client.subscribe('warehouse_stock_list', { warehouse_id: '...' });
// result = { params_hash: "abc123", version: 1 }
```

### Шаг 4: Обработка real-time обновлений

Centrifugo доставляет три типа сообщений:

1. **view_snapshot** — полный snapshot при подписке
2. **table_diff** — изменение данных конкретной записи (JSON Patch)
3. **view_diff** — изменение состава view (добавление/удаление записей)

Svelte 5 proxies автоматически обновляют UI при изменении данных в tableStore.

## Визуализация конкретных сценариев

### «Складские остатки в реальном времени»
- `client.subscribe('warehouse_stock_list', { warehouse_id })` → начальные данные
- При приёмке/отгрузке → `table_diff` с обновлённым quantity → UI обновляется автоматически

### «Машина с грузом едет на склад»
- Entity kind=vehicle + component geo (lat, lng, speed, heading)
- Event `logistics.geo.updated` → view_diff с обновлёнными координатами
- Фронтенд: маркер на карте, линия маршрута из route_stops

### «Приготовление Американо в кофейне»
- Order entity → `client.subscribe('order_detail', { order_id })`
- Process transition events → order status обновляется через table_diff
- Timeline из process_instances.history

### «Перетекание товара от блока к блоку»
- `warehouse.stock.shipped` → view_diff (удаление из source warehouse view)
- `warehouse.stock.received` → view_diff (добавление в dest warehouse view)
- Оба обновления приходят автоматически через triggers

## Модель данных (что фронтенд получает)

### View Snapshot (через Centrifugo)
```json
{
    "type": "view_snapshot",
    "view": "warehouse_stock_list",
    "params_hash": "abc123",
    "version": 1,
    "refs": [
        {"table": "stock_levels", "id": "uuid", "fields": ["quantity", "product_name"]}
    ],
    "tables": {
        "stock_levels": {
            "uuid": {"quantity": 100, "product_name": "Кофе Арабика"}
        }
    }
}
```

### Entity (через REST)
```json
{
    "id": "uuid", "workspace_id": "uuid", "kind": "product",
    "name": "Кофе Арабика", "status": "active", "parent_id": null,
    "meta": {}, "created_at": "ISO8601", "updated_at": "ISO8601",
    "components": [ { "type": "price", "data": {...}, "version": 3 } ]
}
```

## Пагинация
`?limit=50&offset=0&sort=created_at&order=desc`
Ответ: `{ "items": [...], "total": N, "limit": 50, "offset": 0 }`

## Ошибки
```json
{ "code": "NOT_FOUND", "message": "entity not found" }
```
HTTP: 200, 201, 204, 400, 401, 403, 404, 409, 500.

## Аутентификация
Cookie sessions: `teco_session` (72h) + `teco_seance` (10m auto-refresh). Все запросы с `credentials: 'include'`.
Workspace ID хранится в session (Redis). Для смены workspace — `POST /auth/switch`.
При истечении seance (10m) — `POST /auth/unlock` для продления.
