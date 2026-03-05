# Process Engine — DSL и State Machine

## 1. Что это

Движок бизнес-процессов, исполняющий конечные автоматы (finite state machines), описанные в YAML. Каждый процесс привязан к Entity и управляет его жизненным циклом через реакцию на события.

## 2. YAML Schema

```yaml
id: string              # уникальный ID определения (snake_case)
name: string            # человекочитаемое имя
description: string     # описание
trigger_on: string      # тип события, запускающего процесс (напр. "order.created")
entity_kind: string     # kind сущности, к которой привязан процесс
init_state: string      # начальное состояние

states:
  state_name:                     # ключ = ID состояния
    name: string                  # человекочитаемое имя
    terminal: bool                # true = конечное состояние
    timeout:                      # опционально: автопереход по таймауту
      duration: string            # "24h", "30m", "7d"
      to: string                  # состояние перехода
    on_enter:                     # действия при входе в состояние
      - type: string              # emit_event | update_status | notify | webhook | update_component
        params: map
    on_exit:                      # действия при выходе из состояния
      - type: string
        params: map
    transitions:                  # список возможных переходов
      - to: string                # целевое состояние
        event: string             # тип события-триггера
        condition:                # опционально: условие перехода
          field: string           # поле в event.data
          operator: string        # eq, neq, gt, lt, gte, lte, in, not_in, exists
          value: any
        actions:                  # действия при переходе
          - type: string
            params: map
```

## 3. Типы действий (actions)

### emit_event
Публикует событие в Event Bus.
```yaml
- type: emit_event
  params:
    event_type: "notification.order_confirmed"
    data:
      message: "Заказ подтверждён"
      severity: "info"
```

### update_status
Обновляет status сущности через Event Bus.
```yaml
- type: update_status
  params:
    status: "processing"
```

### update_component
Обновляет компонент сущности.
```yaml
- type: update_component
  params:
    component_type: "delivery"
    data:
      status: "in_transit"
```

### notify
Создаёт уведомление (внутреннее событие для фронтенда).
```yaml
- type: notify
  params:
    channel: "workspace"      # workspace | user | role
    target: "manager"         # role name или user_id
    title: "Новый заказ"
    body: "Заказ №{number} ожидает подтверждения"
    severity: "info"          # info, warning, error, success
```

### webhook
Вызов внешнего HTTP endpoint.
```yaml
- type: webhook
  params:
    url: "https://example.com/hook"
    method: "POST"
    headers:
      X-Secret: "abc"
    body_template: '{"order_id": "{entity_id}"}'
    timeout: "10s"
    retry: 3
```

## 4. Условия (conditions)

### Операторы

| Operator | Описание | Пример |
|---|---|---|
| `eq` | Равно | `{field: "status", operator: "eq", value: "paid"}` |
| `neq` | Не равно | `{field: "type", operator: "neq", value: "draft"}` |
| `gt` | Больше | `{field: "total", operator: "gt", value: 100000}` |
| `lt` | Меньше | |
| `gte` | Больше или равно | |
| `lte` | Меньше или равно | |
| `in` | В списке | `{field: "status", operator: "in", value: ["paid", "confirmed"]}` |
| `not_in` | Не в списке | |
| `exists` | Поле существует | `{field: "tracking_number", operator: "exists", value: true}` |

Условие проверяется по `event.data`. Если условие не выполнено — переход не происходит.

## 5. Жизненный цикл процесса

1. Event Bus получает событие с типом, совпадающим с `trigger_on` определения.
2. Process Engine создаёт `ProcessInstance` с `current_state = init_state`.
3. Выполняются `on_enter` действия начального состояния.
4. При каждом новом событии для entity_id Process Engine проверяет все активные instances:
   a. Берёт текущее состояние instance.
   b. Перебирает transitions текущего состояния.
   c. Проверяет event.type == transition.event.
   d. Проверяет condition (если есть).
   e. Первый подходящий transition выполняется.
5. При переходе: on_exit → actions → обновление current_state → on_enter нового состояния.
6. Если новое состояние terminal → status = 'completed'.

## 6. Валидация определений

При загрузке YAML проверяется:
- `id` не пустой, уникальный.
- `init_state` существует в `states`.
- Все `transitions[].to` ведут в существующие состояния.
- Есть хотя бы одно терминальное состояние.
- Все `transitions` имеют `event`.
- `condition.operator` — из списка допустимых.
- `actions[].type` — из списка допустимых.
- Нет циклов без выхода (все пути ведут к терминальному состоянию — warning, не error).

## 7. Определения процессов

### order_fulfillment.yaml
Полный цикл заказа: new → confirmed → paid → assembling → shipped → delivered | cancelled.

### stock_replenishment.yaml
При отгрузке: проверка остатка → если ниже минимума → алерт.

### employee_onboarding.yaml
Приём сотрудника: документы → доступы → обучение → активен.

### delivery_tracking.yaml
Доставка: назначена → в пути → прибыл → завершена.

Конкретные YAML-файлы — в `/processes/`. Генерировать по описанию выше.

## 8. API для процессов

```
GET  /api/v1/workspaces/{wsID}/processes/definitions         — список определений
GET  /api/v1/workspaces/{wsID}/processes/definitions/{defID} — одно определение
GET  /api/v1/workspaces/{wsID}/processes/instances            — активные instances
GET  /api/v1/workspaces/{wsID}/processes/instances/{instID}   — instance + history
GET  /api/v1/workspaces/{wsID}/processes/entity/{entityID}    — instances для entity
POST /api/v1/workspaces/{wsID}/processes/trigger              — ручной trigger события
```

## 9. Process History

В `process_instances.history` (JSONB array) сохраняется каждый переход:

```json
[
    {"from": "new", "to": "confirmed", "event": "order.confirmed", "timestamp": "..."},
    {"from": "confirmed", "to": "paid", "event": "order.paid", "timestamp": "..."}
]
```

Это даёт фронтенду полную timeline процесса для визуализации.
