# Просто попробовал claude - сделай тесты..... Работает, но потом разберусь

# Интеграционные тесты для AT API

## Описание

HTTP интеграционные тесты для всех API endpoints. Тесты выполняют реальные HTTP запросы к запущенному API сервису и проверяют ответы.

**Важно:** Тесты работают с реальной БД, которая должна быть предварительно настроена через миграции.

## Что тестируется

### ✅ GET /health
- `TestHealthCheck` - проверка health endpoint

### ✅ POST /api/v1/tasks (создание задания)
- `TestCreateTask` - успешное создание задания
- `TestCreateTaskInvalidData` - проверка валидации (отсутствие полей, execute_at в прошлом)

### ✅ GET /api/v1/tasks/:id (получение задания)
- `TestGetTaskNotFound` - получение несуществующего задания (404)

### ✅ DELETE /api/v1/tasks/:id (отмена задания)
- Проверяется в `TestFullCycle`

### ✅ GET /api/v1/tasks (список заданий)
- `TestListTasks` - получение списка заданий
- `TestListTasksWithFilters` - фильтрация по task_type
- `TestListTasksWithPagination` - пагинация (limit/offset)

### ✅ Полный цикл
- `TestFullCycle` - создание → получение → отмена задания

## Подготовка к запуску

### 1. Запустить PostgreSQL и применить миграции

```bash
# Запустите PostgreSQL (через Docker или локально)
docker-compose up -d postgres

# Или используйте существующую БД
# Убедитесь, что миграции применены (таблица scheduled_tasks существует)
psql -U postgres -d at_scheduler -f ../sql/ddl.sql
```

### 2. Запустить at-api сервис

```bash
# В одном терминале запустите API
cd at-api/src
go run main.go

# API должен быть доступен на http://localhost:8080
```

## Запуск тестов

### Базовый запуск

```bash
cd at-api/tests
go test -v
```

### С custom URL API

Если API запущен на другом порту:

```bash
cd at-api/tests
API_URL=http://localhost:9090 go test -v
```

### Запуск конкретного теста

```bash
cd at-api/tests
go test -v -run TestHealthCheck
go test -v -run TestFullCycle
```

### Краткий вывод (без verbose)

```bash
cd at-api/tests
go test
```

## Примеры вывода

### Успешный запуск

```
=== RUN   TestHealthCheck
    api_test.go:64: Testing GET /health
    api_test.go:76: ✅ Health check passed
--- PASS: TestHealthCheck (0.01s)
=== RUN   TestCreateTask
    api_test.go:80: Testing POST /api/v1/tasks
    api_test.go:128: ✅ Task created successfully with ID=42
--- PASS: TestCreateTask (0.05s)
=== RUN   TestFullCycle
    api_test.go:203: Testing full cycle: create -> get -> cancel
    api_test.go:228: Created task ID=43
    api_test.go:252: ✅ Got task ID=43, status=pending
    api_test.go:276: ✅ Task cancelled successfully, status=cancelled
--- PASS: TestFullCycle (0.15s)
PASS
ok      at-api/tests    0.210s
```

### Если API не запущен

```
=== RUN   TestHealthCheck
    api_test.go:67: Failed to call health endpoint: Get "http://localhost:8080/health": dial tcp [::1]:8080: connect: connection refused
--- FAIL: TestHealthCheck (0.00s)
```

**Решение:** Запустите at-api сервис перед запуском тестов.

## Структура тестов

### api_test.go

Основной файл с тестами:

- **Структуры данных** - Task, TaskResponse, TaskListResponse, ErrorResponse
- **Вспомогательные функции** - getEnv для конфигурации
- **Тестовые функции** - TestXxx для каждого сценария

Каждый тест:
1. Выполняет HTTP запрос к API
2. Проверяет статус код ответа
3. Парсит JSON ответ
4. Проверяет содержимое данных
5. Выводит результат с ✅/❌

## Конфигурация

### Переменные окружения

- `API_URL` - базовый URL API (по умолчанию: `http://localhost:8080`)

Пример:
```bash
export API_URL=http://api.example.com
go test -v
```

## Интеграция с CI/CD

### GitHub Actions пример

```yaml
name: API Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest

    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_DB: at_scheduler
          POSTGRES_USER: postgres
          POSTGRES_PASSWORD: postgres
        ports:
          - 5432:5432

    steps:
      - uses: actions/checkout@v3

      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Apply migrations
        run: psql -h localhost -U postgres -d at_scheduler -f sql/ddl.sql
        env:
          PGPASSWORD: postgres

      - name: Start API
        run: |
          cd at-api/src
          go run main.go &
          sleep 5
        env:
          DB_HOST: localhost
          DB_PORT: 5432
          DB_USER: postgres
          DB_PASSWORD: postgres
          DB_NAME: at_scheduler

      - name: Run tests
        run: |
          cd at-api/tests
          go test -v
```

## Makefile для удобства

Создайте `Makefile` в корне at-api:

```makefile
.PHONY: test test-api start-api

# Запустить API в фоновом режиме
start-api:
	@echo "Starting API..."
	@cd src && go run main.go &
	@sleep 2
	@echo "API started on http://localhost:8080"

# Запустить тесты
test-api:
	@echo "Running API integration tests..."
	@cd tests && go test -v

# Запустить всё: API + тесты
test: start-api test-api
	@echo "All tests completed"
```

Использование:
```bash
make test-api  # Только тесты (API уже должен быть запущен)
make start-api # Запустить API
```

## Troubleshooting

### Ошибка: connection refused

API не запущен или запущен на другом порту.

**Решение:**
```bash
# Проверьте, что API запущен
curl http://localhost:8080/health

# Или укажите правильный URL
API_URL=http://localhost:9090 go test -v
```

### Ошибка: 404 Not Found

Проверьте правильность URL в тестах и что роутинг API настроен корректно.

### Тесты создают слишком много данных в БД

После тестов можно очистить тестовые данные:

```sql
-- Удалить все задания типа *_test*
DELETE FROM scheduled_tasks WHERE task_type LIKE '%test%';
```

Или добавить cleanup в тесты:
```go
func TestMain(m *testing.M) {
	// Cleanup перед тестами
	cleanupTestData()

	code := m.Run()

	// Cleanup после тестов
	cleanupTestData()

	os.Exit(code)
}
```

## Дополнительные возможности

### Тестирование с разными окружениями

```bash
# Тестирование локального API
API_URL=http://localhost:8080 go test -v

# Тестирование staging
API_URL=https://staging.example.com go test -v

# Тестирование production (будьте осторожны!)
API_URL=https://api.example.com go test -v
```

### Параллельный запуск тестов

По умолчанию Go тесты запускаются последовательно. Для параллельного запуска:

```go
func TestExample(t *testing.T) {
	t.Parallel()  // Добавить в каждый тест
	// ...
}
```

Но будьте осторожны с конкурентным доступом к БД!

## Расширение тестов

Чтобы добавить новый тест:

1. Создайте функцию `TestXxx` в `api_test.go`
2. Выполните HTTP запрос с помощью `http.Get`, `http.Post` и т.д.
3. Проверьте статус код и содержимое ответа
4. Используйте `t.Log`, `t.Error`, `t.Fatal` для вывода результатов

Пример:
```go
func TestMyNewFeature(t *testing.T) {
	t.Log("Testing my new feature")

	resp, err := http.Get(apiURL + "/api/v1/my-endpoint")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status: got=%d, want=200", resp.StatusCode)
	}

	t.Log("✅ Test passed")
}
```
