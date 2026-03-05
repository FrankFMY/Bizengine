# Real-time Protocol (Centrifugo)

## Архитектура

Real-time обновления доставляются через **Centrifugo v6** — выделенный WebSocket сервер. BizEngine не реализует собственный WebSocket hub. Вместо этого:

1. Клиент подключается к Centrifugo по WebSocket.
2. Centrifugo проксирует connect/subscribe запросы на BizEngine для авторизации.
3. BizEngine публикует сообщения в Centrifugo через HTTP API.

## Подключение

```
ws://centrifugo-host:8001/connection/websocket
```

Клиент использует Centrifugo SDK (centrifuge-js). Авторизация — через proxy: Centrifugo пересылает cookies клиента на BizEngine.

## Proxy Endpoints (внутренние)

### Connect Proxy

```
POST /api/internal/centrifugo/connect
```

Centrifugo вызывает при подключении клиента. BizEngine валидирует cookies (`teco_session` + `teco_seance`), возвращает user ID.

### Subscribe Proxy

```
POST /api/internal/centrifugo/subscribe
```

Centrifugo вызывает при подписке на канал. BizEngine проверяет что пользователь имеет доступ к workspace канала.

## Каналы

| Канал | Формат | Назначение |
|---|---|---|
| `org:<orgID>` | `org:uuid` | Table diffs — row-level data changes broadcast to all users in organization |
| `workspace:<orgID>` | `workspace:uuid` | Workspace-wide events (presence, status) |
| `views:<seanceID>` | `views:uuid` | View diffs — structural changes for a specific browser session |

Namespace настройки в `deploy/centrifugo.json`:
- `org`: subscribe proxy enabled, history enabled
- `workspace`: presence enabled, subscribe proxy enabled
- `views`: presence disabled, subscribe proxy enabled, history disabled

## Views API (REST)

Управление подписками на views — через REST API, не через Centrifugo напрямую:

```
POST /api/v1/workspaces/{wsID}/views/subscribe     — подписаться на view
POST /api/v1/workspaces/{wsID}/views/unsubscribe    — отписаться от view
GET  /api/v1/workspaces/{wsID}/views/active          — список активных подписок
POST /api/v1/workspaces/{wsID}/views/sync             — синхронизация после reconnect
```

## Server → Client (через Centrifugo)

### view_snapshot

Отправляется при первой подписке на view. Содержит полный snapshot.

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

### table_diff

Отправляется когда изменяется запись, на которую ссылается view.

```json
{
    "type": "table_diff",
    "table": "stock_levels",
    "id": "uuid",
    "ver": 2,
    "patch": [
        {"op": "replace", "path": "/quantity", "value": 95}
    ]
}
```

### view_diff

Отправляется когда изменяется состав view (добавление/удаление записей).

```json
{
    "type": "view_diff",
    "view": "warehouse_stock_list",
    "params_hash": "abc123",
    "version": 2,
    "refs_patch": [
        {"op": "add", "path": "/3", "value": {"table": "stock_levels", "id": "new-uuid", "fields": ["quantity"]}}
    ],
    "tables": {
        "stock_levels": {
            "new-uuid": {"quantity": 50, "product_name": "Чай зелёный"}
        }
    }
}
```

## Конфигурация Centrifugo

Файл: `deploy/centrifugo.json`

Ключевые настройки:
- `http_api.key` — API key для публикации сообщений
- `client.proxy.connect` — проксирование подключений на BizEngine
- `channel.proxy.subscribe` — проксирование подписок
- `channel.namespaces` — настройки каналов workspace и views

## Reconnection (клиентская сторона)

1. Centrifugo SDK автоматически переподключается с exponential backoff.
2. После reconnect → `POST /views/sync` с текущими versions → сервер отправляет недостающие diffs или полные snapshots.
3. Sync API принимает как view versions, так и table versions — возвращает только изменения.

## SDK

TypeScript SDK для интеграции с Svelte 5:
- `sdk/views.d.ts` — типы для всех 20 views (автогенерация из Go через `scripts/gen_view_types.go`)
- `sdk/client.ts` — ArcanaClient с typed subscribe/unsubscribe/sync и message handlers
