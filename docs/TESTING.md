# Стратегия тестирования

## Уровни тестов

### 1. Unit-тесты (каждый сервис)

Что тестировать: бизнес-логику сервисов с замоканными repository.

```go
// warehouse/service_test.go
func TestReceive_PositiveQuantity(t *testing.T) { ... }
func TestReceive_NegativeQuantity_Error(t *testing.T) { ... }
func TestShip_InsufficientStock_Error(t *testing.T) { ... }
func TestTransfer_SameWarehouse_Error(t *testing.T) { ... }
```

Mock repositories через интерфейсы. Использовать `testify/mock` или ручные моки.

Покрытие: core/ — 70%+, module/ — 50%+.

### 2. Integration-тесты (repository + PostgreSQL)

Что тестировать: SQL запросы, индексы, constraints, RLS.

```go
// storage/postgres/entity_repo_test.go (с testcontainers)
func TestEntityRepo_Create_And_Get(t *testing.T) { ... }
func TestEntityRepo_WorkspaceIsolation(t *testing.T) { ... }
func TestEntityRepo_SoftDelete(t *testing.T) { ... }
```

Реальный PostgreSQL через testcontainers-go. Миграции прогоняются перед тестами.

### 3. Event Bus тесты

```go
func TestBus_PublishAndSubscribe(t *testing.T) { ... }
func TestBus_WildcardSubscription(t *testing.T) { ... }
func TestBus_EventPersisted(t *testing.T) { ... }
```

### 4. Process Engine тесты

```go
func TestEngine_OrderFulfillment_HappyPath(t *testing.T) { ... }
func TestEngine_OrderFulfillment_Cancellation(t *testing.T) { ... }
func TestEngine_ConditionEvaluation(t *testing.T) { ... }
func TestEngine_InvalidDefinition_Error(t *testing.T) { ... }
```

Загрузка реальных YAML из /processes, прогон через engine с mock event bus.

### 5. API тесты (handler level)

```go
func TestCreateEntity_Success(t *testing.T) { ... }
func TestCreateEntity_InvalidBody_400(t *testing.T) { ... }
func TestCreateEntity_Unauthorized_401(t *testing.T) { ... }
func TestCreateEntity_Forbidden_403(t *testing.T) { ... }
```

Использовать `httptest.NewRecorder()` + chi router.

## Структура тестовых файлов

Тесты рядом с кодом: `service.go` → `service_test.go`.

## Тестовые утилиты

Создать пакет `internal/testutil/`:
- `testutil.NewTestDB(t *testing.T) *pgxpool.Pool` — testcontainers Postgres + миграции.
- `testutil.NewTestWorkspace(t, pool) (wsID, userID)` — seed workspace + user.
- `testutil.NewTestEntity(t, pool, wsID, kind, name) entityID` — seed entity.

## Запуск

```bash
make test              # unit-тесты (быстрые)
make test-integration  # с testcontainers (требует Docker)
make test-coverage     # отчёт покрытия
```

## CI

В GitHub Actions:
1. `go vet ./...`
2. `go test ./... -race -count=1`
3. Integration tests с PostgreSQL service container.
