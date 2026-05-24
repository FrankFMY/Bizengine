# BizEngine SDK Quick Reference

Quick reference for frontend development. All endpoints use cookies for auth (`credentials: 'include'`).

## 1. How to Subscribe to a Reactive View

### Subscribe via Arcana

```
POST /arcana/subscribe
Content-Type: application/json

{"view": "catalog_products_list", "params": {"search": "widget", "limit": 20}}
```

Response:

```json
{
  "ok": true,
  "data": {
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

The response contains the full initial dataset. Store `params_hash` to unsubscribe later. Store `version` for sync on reconnect.

### Unsubscribe

```
POST /arcana/unsubscribe
{"params_hash": "a1b2c3d4e5f67890"}
```

### Sync (reconnect)

```
POST /arcana/sync
{"views": [{"view": "catalog_products_list", "params_hash": "a1b2c3d4e5f67890", "version": 42}]}
```

### Available Graphs

38 graphs: `catalog_products_list`, `catalog_product_detail`, `catalog_categories_tree`, `warehouse_stock_list`, `warehouse_stock_detail`, `warehouse_low_stock`, `warehouse_inventory_detail`, `orders_list`, `order_detail`, `order_refunds_list`, `orders_dashboard`, `hr_employees_list`, `hr_employee_detail`, `hr_shifts_schedule`, `hr_timesheets_list`, `hr_payroll_list`, `hr_absences_list`, `finance_trial_balance`, `finance_transactions_list`, `finance_account_balance`, `finance_pnl`, `finance_periods_list`, `finance_cash_operations`, `logistics_routes_list`, `logistics_route_detail`, `logistics_vehicles_map`, `crm_customers_list`, `crm_customer_detail`, `crm_suppliers_list`, `organization_settings`, `notifications_list`, `notifications_unread`, `dashboard_summary`, `bank_reconciliation_list`, `bank_reconciliation_detail`, `chat_conversations_list`, `chat_messages`, `chat_unread_total`.

Full schema: `GET /arcana/schema`.

## 2. Real-Time Message Formats

Connect to Centrifugo WebSocket at `ws://localhost:8001/connection/websocket`. Auth is handled by Centrifugo connect proxy (`POST /api/internal/centrifugo/connect`).

### Channels

- `workspace:{organization_id}` — subscribe for Arcana `table_diff` messages (row-level data changes, organization-wide)
- `org:{organization_id}` — subscribe for raw BizEngine domain events
- `views:{seance_id}` — subscribe for `view_snapshot` and `view_diff` messages (per-seance)
- `chat:{conversation_id}` — subscribe for real-time chat messages and typing indicators in a specific conversation

Get `seance_id` and `organization_id` from `GET /api/v1/auth/check`.

### table_diff

Sent to `org:{id}` when a row's data changes. Apply as JSON Patch to your local normalized store.

```json
{
  "type": "table_diff",
  "data": {
    "table": "orders",
    "id": "order-uuid",
    "ver": 3,
    "patch": [
      {"op": "replace", "path": "/status", "value": "shipped"}
    ]
  }
}
```

### view_diff

Sent to `views:{seanceId}` when a view's ref list changes (items added/removed).

```json
{
  "type": "view_diff",
  "data": {
    "view": "orders_list",
    "params_hash": "a1b2c3d4e5f67890",
    "version": 43,
    "refs_patch": [
      {"op": "add", "path": "/5", "value": {"table": "orders", "id": "new-uuid", "fields": ["status", "total"]}},
      {"op": "replace", "path": "/2", "value": null}
    ],
    "tables": {
      "orders": {
        "new-uuid": {"id": "new-uuid", "status": "draft", "total": 150000}
      }
    }
  }
}
```

`refs_patch`: `add` = new item added to view, `replace` with `null` = item removed from view. `tables` contains data only for newly added refs.

### view_snapshot

Sent to `views:{seanceId}` on initial subscribe (full data).

```json
{
  "type": "view_snapshot",
  "data": {
    "view": "catalog_products_list",
    "params_hash": "a1b2c3d4e5f67890",
    "version": 42,
    "refs": [{"table": "catalog_products", "id": "uuid-1", "fields": ["name", "sku"]}],
    "tables": {"catalog_products": {"uuid-1": {"id": "uuid-1", "name": "Widget"}}}
  }
}
```

## 3. /pass/temp — Quick Dev Setup

Creates user + phone + organization + labor in one call. No auth required. Sets cookies.

```
POST /api/v1/pass/temp
Content-Type: application/json

{
  "data": {
    "country": "RU",
    "phone": "+7 (999) 123-45-67",
    "secret": "my_password",
    "name": "Test Company",
    "inn": "1234567890"
  }
}
```

Response: `{"ok": true}` with `teco_session` + `teco_seance` cookies set.

After this call you are fully authenticated as owner of the organization. You can immediately call any API endpoint or subscribe to Arcana graphs.

Calling again with the same phone/INN reuses existing records and updates the secret.

## 4. /auth/check for SSR

```
GET /api/v1/auth/check
Cookie: teco_session=...; teco_seance=...
```

Response:

```json
{
  "ok": true,
  "data": {
    "seance_id": "seance-uuid",
    "session_id": "session-uuid",
    "user_id": "user-uuid",
    "organization_id": "org-uuid",
    "phone_id": "phone-uuid",
    "admin": true
  }
}
```

Use this for:
- SSR auth check: forward the cookies from the browser request to BizEngine, check `ok` field.
- Getting `seance_id` and `organization_id` for Centrifugo channel subscriptions.
- `admin` is `true` when role is `owner` or `admin`.

On 401: check the response body (`"SESSION"` or `"SEANCE"`) to decide whether to re-login or unlock.

## 5. Cookie Format

| Cookie | TTL | Properties |
|--------|-----|------------|
| `teco_session` | 72 hours | HttpOnly, SameSite=Lax, Secure (prod), Path=/ |
| `teco_seance` | 10 minutes (sliding) | HttpOnly, SameSite=Lax, Secure (prod), Path=/ |

Both cookies are set on: register, login, unlock, /pass/temp.

Client-side: you never read these cookies directly (HttpOnly). Just set `credentials: 'include'` on all fetch calls and the browser handles them.

Seance auto-extends on every authenticated request. If you stop making requests for 10 minutes, the seance expires — call `POST /api/v1/auth/unlock` with `{"password": "..."}` to get a new one.

## 6. Error Format

### Standard API errors (BizEngine REST)

```json
{
  "ok": false,
  "error": {
    "code": "NOT_FOUND",
    "details": "entity not found"
  }
}
```

Error codes: `BAD_REQUEST`, `UNAUTHORIZED`, `FORBIDDEN`, `NOT_FOUND`, `CONFLICT`, `UNPROCESSABLE`, `LOCKED`, `RATE_LIMITED`, `INTERNAL_ERROR`, `MAINTENANCE`, `GATEWAY_TIMEOUT`.

### Validation errors

```json
{
  "ok": false,
  "validation": {
    "email": "required",
    "password": "min length 8"
  }
}
```

HTTP status: 422.

### Auth 401 responses

The auth middleware returns `Content-Type: text/plain` (not JSON):
- Body `"SESSION"` — session expired or missing. Redirect to login.
- Body `"SEANCE"` — seance expired. Call `POST /api/v1/auth/unlock`.

### Arcana errors

```json
{"ok": false, "error": "graph \"nonexistent\" not found"}
```

HTTP statuses: 400 (bad params), 404 (graph not found), 409 (unauthorized/forbidden), 429 (too many subscriptions), 500 (internal).

## 7. Banking

White-label banking integration: initiate payments, check statuses, auto-reconcile bank transactions, and process payroll.

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/organizations/{orgID}/white-label` | Get white-label banking config |
| POST | `/api/v1/organizations/{orgID}/payments/initiate` | Initiate a payment |
| GET | `/api/v1/organizations/{orgID}/payments/{paymentID}/status` | Check payment status |
| POST | `/api/v1/organizations/{orgID}/finance/bank/auto-reconcile` | Auto-reconcile bank transactions |
| GET | `/api/v1/organizations/{orgID}/finance/bank/reconciliations` | List reconciliation runs |
| POST | `/api/v1/organizations/{orgID}/hr/payroll/{year}/{month}/pay` | Execute payroll payment |

### Bank OAuth

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/auth/bank/{bankCode}/login` | Redirect to bank OAuth login |
| GET | `/api/v1/auth/bank/{bankCode}/callback` | Bank OAuth callback |

### Payment Webhook

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/webhooks/bank/payment-callback` | Receive payment status updates from bank |

### Initiate Payment

```
POST /api/v1/organizations/{orgID}/payments/initiate
Content-Type: application/json

{
  "data": {
    "recipient_account": "40702810938000012345",
    "recipient_bik": "044525225",
    "recipient_name": "OOO Supplier",
    "recipient_inn": "7707083893",
    "amount": 1500000,
    "currency": "RUB",
    "purpose": "Payment for invoice #INV-2026-0042"
  }
}
```

Response:

```json
{
  "ok": true,
  "data": {
    "payment_id": "pay-uuid-1234",
    "status": "pending",
    "created_at": "2026-03-15T10:30:00Z"
  }
}
```

### Auto-Reconcile

```
POST /api/v1/organizations/{orgID}/finance/bank/auto-reconcile
Content-Type: application/json

{
  "data": {
    "bank_account_id": "acc-uuid-5678",
    "date_from": "2026-03-01",
    "date_to": "2026-03-15"
  }
}
```

Response:

```json
{
  "ok": true,
  "data": {
    "reconciliation_id": "rec-uuid-9012",
    "matched": 47,
    "unmatched": 3,
    "auto_matched": 44,
    "manual_review": 3,
    "status": "completed"
  }
}
```

## 8. Messenger

Built-in organization chat. Supports direct messages, group conversations, and entity-linked conversations (auto-created for orders and routes).

### Arcana Graphs

Subscribe to these graphs for reactive chat data:

| Graph | Params | Description |
|-------|--------|-------------|
| `chat_conversations_list` | `{ "limit": 50, "offset": 0 }` | Paginated list of conversations for the current user |
| `chat_messages` | `{ "conversation_id": "uuid", "limit": 50, "before": "msg-uuid" }` | Messages in a conversation, cursor-paginated |
| `chat_unread_total` | `{}` | Total unread message count across all conversations |

### Real-Time

Subscribe to Centrifugo channel `chat:{conversationID}` for real-time message delivery and typing indicators in a specific conversation. Messages arrive as:

```json
{
  "type": "chat_message",
  "data": {
    "id": "msg-uuid",
    "conversation_id": "conv-uuid",
    "sender_id": "user-uuid",
    "content": "Hello!",
    "content_type": "text",
    "created_at": "2026-03-15T10:30:00Z"
  }
}
```

Typing indicator:

```json
{
  "type": "chat_typing",
  "data": {
    "conversation_id": "conv-uuid",
    "user_id": "user-uuid"
  }
}
```

### REST Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/organizations/{orgID}/messenger/conversations` | List conversations |
| POST | `/api/v1/organizations/{orgID}/messenger/conversations` | Create conversation |
| GET | `/api/v1/organizations/{orgID}/messenger/conversations/{convID}` | Get conversation details |
| POST | `/api/v1/organizations/{orgID}/messenger/conversations/{convID}/members` | Add member to conversation |
| DELETE | `/api/v1/organizations/{orgID}/messenger/conversations/{convID}/members/{userID}` | Remove member |
| POST | `/api/v1/organizations/{orgID}/messenger/conversations/{convID}/mute` | Mute/unmute conversation |
| GET | `/api/v1/organizations/{orgID}/messenger/conversations/{convID}/messages` | List messages |
| POST | `/api/v1/organizations/{orgID}/messenger/conversations/{convID}/messages` | Send message |
| PUT | `/api/v1/organizations/{orgID}/messenger/conversations/{convID}/messages/{msgID}` | Edit message |
| DELETE | `/api/v1/organizations/{orgID}/messenger/conversations/{convID}/messages/{msgID}` | Delete message |
| POST | `/api/v1/organizations/{orgID}/messenger/conversations/{convID}/read` | Mark conversation as read |
| GET | `/api/v1/organizations/{orgID}/messenger/unread-count` | Get total unread count |
| POST | `/api/v1/organizations/{orgID}/messenger/messages/{msgID}/reactions` | Add reaction |
| DELETE | `/api/v1/organizations/{orgID}/messenger/messages/{msgID}/reactions/{emoji}` | Remove reaction |
| POST | `/api/v1/organizations/{orgID}/messenger/conversations/{convID}/typing` | Send typing indicator |
| GET | `/api/v1/organizations/{orgID}/messenger/search` | Search messages |

### Entity Conversations

Conversations are auto-created when an order or route is created. These entity-linked conversations have `entity_type` and `entity_id` set and automatically include relevant participants (assigned employees, drivers, etc.).

### System Messages

System-generated messages (member joined, member left, order status changed) use `content_type='system'` and have structured content in the `metadata` field rather than plain text in `content`.
