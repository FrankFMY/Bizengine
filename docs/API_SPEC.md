# API Specification

Base URL: `/api/v1`
Content-Type: `application/json`
Auth: Cookie-based sessions (`teco_session` + `teco_seance`). Все эндпоинты кроме auth/register, auth/login и health.

## Общие правила

### Пагинация
Все list-эндпоинты: `?limit=50&offset=0&sort=created_at&order=desc`
Response: `{ "items": [...], "total": 1234, "limit": 50, "offset": 0 }`
Default limit: 50. Max limit: 200.

### Ошибки
```json
{ "code": "NOT_FOUND", "message": "entity not found", "details": null }
```
Codes: `BAD_REQUEST`, `UNAUTHORIZED`, `FORBIDDEN`, `NOT_FOUND`, `CONFLICT`, `INTERNAL_ERROR`.

### Include (загрузка связанных данных)
`?include=components` — подгружает компоненты вместе с entity.
`?include=items` — подгружает строки вместе с заказом.

---

## Auth

Cookie-based sessions. Все ответы auth-эндпоинтов устанавливают HttpOnly cookies.

```
POST /api/v1/auth/register
Body: { "email": "", "password": "", "full_name": "" }
Response 201: { "ok": true, "data": { "user": {...}, "workspaces": [...] } }
Sets cookies: teco_session, teco_seance

POST /api/v1/auth/login
Body: { "email": "", "password": "" }
Response 200: { "ok": true, "data": { "user": {...}, "workspaces": [...] } }
Sets cookies: teco_session, teco_seance

POST /api/v1/auth/logout
Response 204
Clears cookies: teco_session, teco_seance

POST /api/v1/auth/unlock
Response 200: { "ok": true }
Refreshes seance cookie (requires valid session cookie only)

POST /api/v1/auth/switch
Body: { "workspace_id": "uuid" }
Response 200: { "ok": true }
Updates workspace_id in session

GET /api/v1/auth/check
Response 200: { "ok": true, "data": { "user_id": "", "workspace_id": "", "role": "" } }
```

## Workspaces

```
POST   /api/v1/workspaces
Body: { "name": "", "slug": "" }
Response 201: Workspace

GET    /api/v1/workspaces
Response 200: [Workspace]  (workspaces текущего пользователя)

GET    /api/v1/workspaces/{wsID}
Response 200: Workspace

PUT    /api/v1/workspaces/{wsID}
Body: { "name": "", "settings": {} }
Response 200: Workspace

POST   /api/v1/workspaces/{wsID}/members
Body: { "email": "", "role": "manager" }
Response 201: Member

GET    /api/v1/workspaces/{wsID}/members
Response 200: [Member]

PUT    /api/v1/workspaces/{wsID}/members/{userID}
Body: { "role": "admin" }
Response 200: Member

DELETE /api/v1/workspaces/{wsID}/members/{userID}
Response 204
```

## Entities (универсальный CRUD)

```
POST   /api/v1/workspaces/{wsID}/entities
Body: { "kind": "product", "name": "...", "parent_id": null, "meta": {} }
Response 201: Entity

GET    /api/v1/workspaces/{wsID}/entities
Query: ?kind=product&status=active&search=кофе&parent_id=uuid&limit=50&offset=0&include=components
Response 200: PageResponse[Entity]

GET    /api/v1/workspaces/{wsID}/entities/{id}
Query: ?include=components
Response 200: Entity (+ components если include)

PUT    /api/v1/workspaces/{wsID}/entities/{id}
Body: { "name": "...", "status": "...", "meta": {} }
Response 200: Entity

DELETE /api/v1/workspaces/{wsID}/entities/{id}
Response 204 (soft delete)
```

## Components

```
PUT    /api/v1/workspaces/{wsID}/entities/{entityID}/components/{type}
Body: { ...component data... }
Response 200: Component

GET    /api/v1/workspaces/{wsID}/entities/{entityID}/components/{type}
Response 200: Component

GET    /api/v1/workspaces/{wsID}/entities/{entityID}/components
Response 200: [Component]

DELETE /api/v1/workspaces/{wsID}/entities/{entityID}/components/{type}
Response 204
```

## Catalog

```
POST   /api/v1/workspaces/{wsID}/catalog/products
Body: { "name": "", "sku": "", "category_id": null, "price": {...}, "barcode": {...}, "attributes": {...} }
Response 201: Product (entity + components)

GET    /api/v1/workspaces/{wsID}/catalog/products
Query: ?category_id=uuid&search=&in_stock=true&limit=50
Response 200: PageResponse[Product]

GET    /api/v1/workspaces/{wsID}/catalog/products/{id}
Response 200: Product (entity + all components)

PUT    /api/v1/workspaces/{wsID}/catalog/products/{id}
Body: { "name": "", "price": {...}, ... }
Response 200: Product

POST   /api/v1/workspaces/{wsID}/catalog/categories
Body: { "name": "", "parent_id": null }
Response 201: Category

GET    /api/v1/workspaces/{wsID}/catalog/categories
Query: ?parent_id=null (null = корневые)
Response 200: [Category] (дерево)
```

## Warehouse

```
POST   /api/v1/workspaces/{wsID}/warehouse/receive
Body: { "product_id": "", "warehouse_id": "", "quantity": 100, "unit": "шт", "cost_per_unit": 15000, "reason": "" }
Response 200: { "movement_id": "", "new_quantity": 150 }

POST   /api/v1/workspaces/{wsID}/warehouse/ship
Body: { "product_id": "", "warehouse_id": "", "quantity": 5, "unit": "шт", "reference_type": "order", "reference_id": "" }
Response 200: { "movement_id": "", "new_quantity": 145 }

POST   /api/v1/workspaces/{wsID}/warehouse/transfer
Body: { "product_id": "", "from_warehouse_id": "", "to_warehouse_id": "", "quantity": 30, "unit": "шт" }
Response 200: { "movement_id": "" }

POST   /api/v1/workspaces/{wsID}/warehouse/adjust
Body: { "product_id": "", "warehouse_id": "", "new_quantity": 100, "reason": "Инвентаризация" }
Response 200: { "movement_id": "", "old_quantity": 95, "new_quantity": 100 }

GET    /api/v1/workspaces/{wsID}/warehouse/{warehouseID}/stock
Query: ?search=&low_stock=true&limit=50
Response 200: PageResponse[StockLevel]

GET    /api/v1/workspaces/{wsID}/warehouse/{warehouseID}/stock/{productID}
Response 200: StockLevel

GET    /api/v1/workspaces/{wsID}/warehouse/low-stock
Response 200: [StockLevel]

GET    /api/v1/workspaces/{wsID}/warehouse/movements
Query: ?product_id=&warehouse_id=&type=receive&since=2024-01-01&limit=50
Response 200: PageResponse[StockMovement]
```

## Orders

```
POST   /api/v1/workspaces/{wsID}/orders
Body: { "customer_id": null, "warehouse_id": "", "items": [{"product_id": "", "quantity": 2, "unit_price": 25000}], "notes": "" }
Response 201: Order (с присвоенным number)

GET    /api/v1/workspaces/{wsID}/orders
Query: ?status=active&customer_id=&since=&until=&search=&limit=50&include=items
Response 200: PageResponse[Order]

GET    /api/v1/workspaces/{wsID}/orders/{id}
Query: ?include=items,process
Response 200: Order

POST   /api/v1/workspaces/{wsID}/orders/{id}/confirm
Response 200: Order

POST   /api/v1/workspaces/{wsID}/orders/{id}/pay
Body: { "amount": 50000, "method": "card" }
Response 200: Order

POST   /api/v1/workspaces/{wsID}/orders/{id}/ship
Body: { "tracking": "" }
Response 200: Order

POST   /api/v1/workspaces/{wsID}/orders/{id}/deliver
Response 200: Order

POST   /api/v1/workspaces/{wsID}/orders/{id}/cancel
Body: { "reason": "" }
Response 200: Order

PUT    /api/v1/workspaces/{wsID}/orders/{id}
Body: { "items": [...], "notes": "" }   // только draft/new заказы
Response 200: Order
```

## HR

```
POST   /api/v1/workspaces/{wsID}/hr/employees
Body: { "name": "", "position": "", "department_id": null, "salary": {...}, "contact": {...} }
Response 201: Employee

GET    /api/v1/workspaces/{wsID}/hr/employees
Query: ?department_id=&status=active&limit=50
Response 200: PageResponse[Employee]

POST   /api/v1/workspaces/{wsID}/hr/shifts
Body: { "employee_id": "", "location_id": "", "start_time": "", "end_time": "", "break_minutes": 60 }
Response 201: Shift

GET    /api/v1/workspaces/{wsID}/hr/shifts
Query: ?employee_id=&location_id=&from=&to=&limit=50
Response 200: PageResponse[Shift]

POST   /api/v1/workspaces/{wsID}/hr/timesheets/clock-in
Body: { "employee_id": "", "shift_id": null }
Response 201: Timesheet

POST   /api/v1/workspaces/{wsID}/hr/timesheets/{id}/clock-out
Response 200: Timesheet
```

## Finance

```
POST   /api/v1/workspaces/{wsID}/finance/transactions
Body: { "date": "2024-01-15", "description": "", "lines": [{"account_id": "", "debit": 50000, "credit": 0}, {"account_id": "", "debit": 0, "credit": 50000}] }
Response 201: Transaction
Validation: sum(debit) == sum(credit)

GET    /api/v1/workspaces/{wsID}/finance/transactions
Query: ?account_id=&from=&to=&limit=50
Response 200: PageResponse[Transaction]

GET    /api/v1/workspaces/{wsID}/finance/accounts
Response 200: [Account] (дерево)

GET    /api/v1/workspaces/{wsID}/finance/accounts/{id}/balance
Query: ?from=&to=
Response 200: { "account_id": "", "debit_total": 0, "credit_total": 0, "balance": 0 }

GET    /api/v1/workspaces/{wsID}/finance/reports/trial-balance
Query: ?date=2024-01-31
Response 200: [{ "account_id": "", "code": "", "name": "", "debit": 0, "credit": 0 }]
```

## Logistics

```
POST   /api/v1/workspaces/{wsID}/logistics/routes
Body: { "name": "", "vehicle_id": "", "driver_id": "", "stops": [{...}], "planned_start": "" }
Response 201: Route

GET    /api/v1/workspaces/{wsID}/logistics/routes
Query: ?status=in_progress&driver_id=&limit=20
Response 200: PageResponse[Route]

POST   /api/v1/workspaces/{wsID}/logistics/routes/{id}/start
Response 200: Route

POST   /api/v1/workspaces/{wsID}/logistics/routes/{id}/stops/{stopID}/arrive
Response 200: RouteStop

POST   /api/v1/workspaces/{wsID}/logistics/routes/{id}/stops/{stopID}/complete
Response 200: RouteStop

POST   /api/v1/workspaces/{wsID}/logistics/geo
Body: { "entity_id": "", "latitude": 55.7558, "longitude": 37.6173, "speed": 60, "heading": 180 }
Response 204

GET    /api/v1/workspaces/{wsID}/logistics/geo/{entityID}/track
Query: ?from=&to=
Response 200: [GeoPoint]
```

## Events (read-only, для аудита и дебага)

```
GET    /api/v1/workspaces/{wsID}/events
Query: ?type=order.created&entity_id=&since=&limit=100
Response 200: PageResponse[Event]

GET    /api/v1/workspaces/{wsID}/events/entity/{entityID}
Query: ?since=&limit=100
Response 200: [Event]
```

## Processes

```
GET    /api/v1/workspaces/{wsID}/processes/definitions
Response 200: [ProcessDefinition]

GET    /api/v1/workspaces/{wsID}/processes/instances
Query: ?status=active&entity_id=&definition_id=
Response 200: PageResponse[ProcessInstance]

GET    /api/v1/workspaces/{wsID}/processes/instances/{id}
Response 200: ProcessInstance (+ history)

GET    /api/v1/workspaces/{wsID}/processes/entity/{entityID}
Response 200: [ProcessInstance]
```

## Views (Reactive)

```
POST /api/v1/workspaces/{wsID}/views/subscribe
Body: { "data": { "view": "warehouse_stock_list", "params": {"warehouse_id": "uuid"} } }
Response 200: { "ok": true, "data": { "params_hash": "abc123", "version": 1 } }

POST /api/v1/workspaces/{wsID}/views/unsubscribe
Body: { "data": { "view": "warehouse_stock_list", "params_hash": "abc123" } }
Response 200: { "ok": true }

GET /api/v1/workspaces/{wsID}/views/active
Response 200: { "ok": true, "data": { "views": [{"view": "", "params_hash": "", "version": 0}] } }

POST /api/v1/workspaces/{wsID}/views/sync
Body: { "data": { "views": [{"view": "", "params_hash": "", "version": 0}], "tables": {"table": {"id": version}} } }
Response 200: { "ok": true }
```

## Internal (Centrifugo Proxy)

Not part of public API. Called by Centrifugo server.

```
POST /api/internal/centrifugo/connect       — validates session cookies
POST /api/internal/centrifugo/subscribe      — validates channel access
```

## Health

```
GET /health
Response 200: { "status": "ok", "postgres": "ok", "redis": "ok" }
No auth required.
```
