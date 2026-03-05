# Arcana Integration

Arcana is a reactive data sync engine that runs in parallel to the legacy views system. It handles graph-based subscriptions, invalidation, and real-time delivery via Centrifugo.

## Architecture

```
BizEngine Server
├── /api/v1/...          (REST API, BizEngine router)
├── /arcana/...          (Arcana HTTP handler, StripPrefix mount)
│   ├── POST /arcana/subscribe
│   ├── POST /arcana/unsubscribe
│   ├── POST /arcana/sync
│   ├── GET  /arcana/active
│   ├── GET  /arcana/schema
│   └── GET  /arcana/health
└── /api/internal/...    (Centrifugo proxy)
```

Arcana is mounted at `/arcana/` via `http.StripPrefix`:

```go
mux.Handle("/arcana/", http.StripPrefix("/arcana", arcanaEngine.Handler()))
```

## Authentication

Arcana uses the same `teco_session` + `teco_seance` cookies as the main API. The `AuthFunc` reads both cookies, validates them against Redis, slides the seance TTL, and returns an `arcana.Identity`:

```go
arcana.Identity{
    SeanceID:    seanceID,
    UserID:      sess.UserID.String(),
    WorkspaceID: sess.OrganizationID.String(),
    Role:        sess.Role,
}
```

Note: Arcana uses `WorkspaceID` internally — it maps to BizEngine's `organization_id`.

## 21 Graph Definitions

Defined in `internal/graphs/`, registered via `graphs.RegisterAll(engine)`.

| Graph Key | Module | Deps | Params |
|-----------|--------|------|--------|
| `catalog_products_list` | Catalog | entities, components | `category_id` (uuid), `search` (string), `limit` (int, default 50), `offset` (int, default 0) |
| `catalog_product_detail` | Catalog | entities, components, stock_levels | `product_id` (uuid, required) |
| `catalog_categories_tree` | Catalog | entities | — |
| `warehouse_stock_list` | Warehouse | stock_levels, entities | `warehouse_id` (uuid), `limit`, `offset` |
| `warehouse_stock_detail` | Warehouse | stock_levels, stock_movements, entities | `product_id` (uuid, required), `warehouse_id` (uuid, required) |
| `warehouse_low_stock` | Warehouse | stock_levels, entities | — |
| `orders_list` | Orders | orders, entities | `status` (string), `limit`, `offset` |
| `order_detail` | Orders | orders, order_items, entities | `order_id` (uuid, required) |
| `orders_dashboard` | Orders | orders | — |
| `hr_employees_list` | HR | entities, components | `limit`, `offset` |
| `hr_employee_detail` | HR | entities, components, shifts | `employee_id` (uuid, required) |
| `hr_shifts_schedule` | HR | shifts, entities | — (current week) |
| `hr_timesheets_list` | HR | timesheets, entities | `limit`, `offset` |
| `finance_trial_balance` | Finance | accounts, transaction_lines | — |
| `finance_transactions_list` | Finance | transactions | `limit`, `offset` |
| `finance_account_balance` | Finance | accounts, transaction_lines | `account_id` (uuid, required) |
| `logistics_routes_list` | Logistics | routes, entities | `status` (string), `limit`, `offset` |
| `logistics_route_detail` | Logistics | routes, route_stops, entities | `route_id` (uuid, required) |
| `logistics_vehicles_map` | Logistics | geo_tracks, entities | — |
| `notifications_unread` | Notifications | notifications | — |
| `dashboard_summary` | Dashboard | orders, stock_levels, entities | — |

## Event to Change Mapping

BizEngine events are converted to Arcana `Change` objects via `graphs.EventToChanges()`. This function maps event type prefixes to table changes:

| Event Prefix | Tables Affected |
|-------------|----------------|
| `entity.*` | entities |
| `component.*` | entities, components |
| `order.*` | orders (+ order_items for order.item events) |
| `catalog.product.*` | entities, components |
| `catalog.category.*` | entities |
| `warehouse.stock.*` | stock_levels (+ stock_movements for received/shipped/adjusted) |
| `hr.employee.*` | entities, components |
| `hr.shift.*` | shifts |
| `hr.timesheet.*` | timesheets |
| `finance.transaction.*` | transactions |
| `finance.account.*` | accounts |
| `logistics.route.*` | routes (+ route_stops for stop events) |
| `logistics.geo.*` | geo_tracks |

The event bus subscriber in `main.go` converts each `types.Event` to changes and calls `arcanaEngine.Notify()`:

```go
eventBus.SubscribeAll(event.SubscriberFunc(func(ctx context.Context, ev types.Event) error {
    data := graphs.EventDataFromJSON(ev.Data)
    if ev.EntityID != nil {
        data["entity_id"] = ev.EntityID.String()
    }
    changes := graphs.EventToChanges(ev.Type, data)
    for _, ch := range changes {
        arcanaEngine.Notify(ctx, ch)
    }
    return nil
}))
```

## Centrifugo Channels

| Channel Pattern | Message Types | Scope |
|----------------|---------------|-------|
| `org:{organization_id}` | `table_diff` | All users in the organization see row-level data changes |
| `views:{seance_id}` | `view_snapshot`, `view_diff` | Per-seance: initial data and ref-list changes |

## Message Formats

### view_snapshot (sent on subscribe)

Published to `views:{seanceID}`.

```json
{
  "type": "view_snapshot",
  "data": {
    "view": "catalog_products_list",
    "params_hash": "a1b2c3d4e5f67890",
    "version": 42,
    "refs": [
      {"table": "catalog_products", "id": "uuid-1", "fields": ["name", "sku", "selling_price"]}
    ],
    "tables": {
      "catalog_products": {
        "uuid-1": {"id": "uuid-1", "name": "Widget", "sku": "W-001", "selling_price": 25000, "status": "active"}
      }
    }
  }
}
```

### table_diff (row data changed)

Published to `org:{organizationID}`.

```json
{
  "type": "table_diff",
  "data": {
    "table": "orders",
    "id": "uuid-1",
    "ver": 3,
    "patch": [
      {"op": "replace", "path": "/status", "value": "shipped"},
      {"op": "replace", "path": "/shipped_at", "value": "2026-03-05T14:00:00Z"}
    ]
  }
}
```

Patch operations follow JSON Patch (RFC 6902): `add`, `replace`, `remove`.

### view_diff (ref list changed)

Published to `views:{seanceID}`.

```json
{
  "type": "view_diff",
  "data": {
    "view": "orders_list",
    "params_hash": "a1b2c3d4e5f67890",
    "version": 43,
    "refs_patch": [
      {"op": "add", "path": "/5", "value": {"table": "orders", "id": "uuid-new", "fields": ["status", "total"]}},
      {"op": "replace", "path": "/2", "value": null}
    ],
    "tables": {
      "orders": {
        "uuid-new": {"id": "uuid-new", "status": "draft", "total": 150000}
      }
    }
  }
}
```

## Subscribing from Frontend

### 1. Subscribe to a graph

```js
const res = await fetch('/arcana/subscribe', {
  method: 'POST',
  credentials: 'include',
  headers: {'Content-Type': 'application/json'},
  body: JSON.stringify({
    view: 'catalog_products_list',
    params: {search: 'widget', limit: 20}
  })
});
const {ok, data} = await res.json();
// data = {params_hash, version, refs, tables}
```

### 2. Connect to Centrifugo and listen for updates

```js
const centrifuge = new Centrifuge('ws://localhost:8001/connection/websocket', {
  // Centrifugo handles auth via connect proxy
});
centrifuge.connect();

// Subscribe to organization-wide table diffs
const orgSub = centrifuge.newSubscription(`org:${organizationId}`);
orgSub.on('publication', ({data}) => {
  if (data.type === 'table_diff') {
    applyTableDiff(data);  // update local normalized store
  }
});
orgSub.subscribe();

// Subscribe to seance-specific view diffs
const viewsSub = centrifuge.newSubscription(`views:${seanceId}`);
viewsSub.on('publication', ({data}) => {
  if (data.type === 'view_snapshot') {
    replaceViewData(data);
  }
  if (data.type === 'view_diff') {
    applyViewDiff(data);
  }
});
viewsSub.subscribe();
```

### 3. Unsubscribe

```js
await fetch('/arcana/unsubscribe', {
  method: 'POST',
  credentials: 'include',
  headers: {'Content-Type': 'application/json'},
  body: JSON.stringify({params_hash: 'a1b2c3d4e5f67890'})
});
```

### 4. Sync on reconnect

```js
await fetch('/arcana/sync', {
  method: 'POST',
  credentials: 'include',
  headers: {'Content-Type': 'application/json'},
  body: JSON.stringify({
    views: [
      {view: 'catalog_products_list', params_hash: 'a1b2c3d4e5f67890', version: 42}
    ]
  })
});
```

### 5. Check active subscriptions

```js
const res = await fetch('/arcana/active', {credentials: 'include'});
// [{graph_key, params_hash, version}, ...]
```

### 6. Get graph schema

```js
const res = await fetch('/arcana/schema', {credentials: 'include'});
// returns registry rep table with all graph definitions
```
