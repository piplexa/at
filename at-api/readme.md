# AT API - REST API для управления запланированными заданиями

REST API сервис для создания, получения, отмены и просмотра запланированных заданий.

## Установка и запуск

### Конфигурация

Создайте файл `.env` в директории `at-api/` на основе `.env.example`:

```bash
cp .env.example .env
```

Отредактируйте `.env` и укажите параметры подключения к PostgreSQL:

```env
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=at_scheduler
DB_SSLMODE=disable
API_PORT=8080
```

### Локальный запуск

```bash
cd at-api/src
go mod download
go build
./at-api
```

### Запуск через Docker Compose

```bash
docker-compose up at-api
```

API будет доступен на `http://localhost:8080`

## API Endpoints

### 1. Создание задания

**POST** `/api/v1/tasks`

Создает новое запланированное задание.

**Тело запроса:**
```json
{
  "execute_at": "2025-11-10T15:00:00Z",
  "task_type": "send_email",
  "payload": {
    "to": "user@example.com",
    "subject": "Напоминание"
  },
  "max_attempts": 3
}
```

**Поля:**
- `execute_at` (обязательное) - время выполнения задания в формате RFC3339 (ISO 8601). Должно быть в будущем.
- `task_type` (обязательное) - тип задания, строка до 50 символов. Используется для маршрутизации задания к обработчику.
- `payload` (обязательное) - данные задания в формате JSON. Любая валидная JSON структура.
- `max_attempts` (опциональное) - максимальное количество попыток выполнения. По умолчанию: 3.

**Ответ (201 Created):**
```json
{
  "task": {
    "id": 1,
    "execute_at": "2025-11-10T15:00:00Z",
    "task_type": "send_email",
    "payload": {
      "to": "user@example.com",
      "subject": "Напоминание"
    },
    "status": "pending",
    "attempts": 0,
    "max_attempts": 3,
    "created_at": "2025-11-10T10:00:00Z",
    "updated_at": "2025-11-10T10:00:00Z"
  }
}
```

**Возможные ошибки:**
- `400 Bad Request` - невалидные данные или execute_at в прошлом
- `500 Internal Server Error` - ошибка при создании задания

---

### 2. Получение задания

**GET** `/api/v1/tasks/:id`

Получает информацию о задании по его ID.

**Параметры URL:**
- `id` - идентификатор задания (число)

**Пример запроса:**
```bash
GET /api/v1/tasks/1
```

**Ответ (200 OK):**
```json
{
  "task": {
    "id": 1,
    "execute_at": "2025-11-10T15:00:00Z",
    "task_type": "send_email",
    "payload": {
      "to": "user@example.com",
      "subject": "Напоминание"
    },
    "status": "completed",
    "attempts": 1,
    "max_attempts": 3,
    "created_at": "2025-11-10T10:00:00Z",
    "updated_at": "2025-11-10T15:01:00Z",
    "completed_at": "2025-11-10T15:01:00Z"
  }
}
```

**Возможные статусы:**
- `pending` - ожидает выполнения
- `processing` - выполняется
- `completed` - успешно выполнено
- `failed` - выполнено с ошибкой (превышено max_attempts)
- `cancelled` - отменено

**Возможные ошибки:**
- `400 Bad Request` - невалидный ID
- `404 Not Found` - задание не найдено
- `500 Internal Server Error` - ошибка при получении задания

---

### 3. Отмена задания

**DELETE** `/api/v1/tasks/:id`

Отменяет задание. Можно отменить только задания в статусе `pending` или `processing`.

**Параметры URL:**
- `id` - идентификатор задания (число)

**Пример запроса:**
```bash
DELETE /api/v1/tasks/1
```

**Ответ (200 OK):**
```json
{
  "task": {
    "id": 1,
    "execute_at": "2025-11-10T15:00:00Z",
    "task_type": "send_email",
    "payload": {
      "to": "user@example.com",
      "subject": "Напоминание"
    },
    "status": "cancelled",
    "attempts": 0,
    "max_attempts": 3,
    "created_at": "2025-11-10T10:00:00Z",
    "updated_at": "2025-11-10T10:05:00Z"
  }
}
```

**Возможные ошибки:**
- `400 Bad Request` - невалидный ID
- `404 Not Found` - задание не найдено или уже выполнено/отменено
- `500 Internal Server Error` - ошибка при отмене задания

---

### 4. Список заданий

**GET** `/api/v1/tasks`

Получает список заданий с фильтрацией и пагинацией.

**Query параметры:**
- `status` (опциональный) - фильтр по статусу: `pending`, `processing`, `completed`, `failed`, `cancelled`
- `task_type` (опциональный) - фильтр по типу задания
- `limit` (опциональный) - количество записей на странице. По умолчанию: 50, максимум: 100
- `offset` (опциональный) - смещение для пагинации. По умолчанию: 0

**Примеры запросов:**
```bash
# Все задания (первые 50)
GET /api/v1/tasks

# Все pending задания
GET /api/v1/tasks?status=pending

# Задания типа send_email
GET /api/v1/tasks?task_type=send_email

# Пагинация: вторая страница по 20 записей
GET /api/v1/tasks?limit=20&offset=20

# Комбинация фильтров
GET /api/v1/tasks?status=pending&task_type=send_email&limit=10
```

**Ответ (200 OK):**
```json
{
  "tasks": [
    {
      "id": 1,
      "execute_at": "2025-11-10T15:00:00Z",
      "task_type": "send_email",
      "payload": {"to": "user@example.com"},
      "status": "pending",
      "attempts": 0,
      "max_attempts": 3,
      "created_at": "2025-11-10T10:00:00Z",
      "updated_at": "2025-11-10T10:00:00Z"
    },
    {
      "id": 2,
      "execute_at": "2025-11-10T16:00:00Z",
      "task_type": "send_sms",
      "payload": {"phone": "+1234567890"},
      "status": "pending",
      "attempts": 0,
      "max_attempts": 3,
      "created_at": "2025-11-10T10:01:00Z",
      "updated_at": "2025-11-10T10:01:00Z"
    }
  ],
  "total": 150
}
```

**Поля ответа:**
- `tasks` - массив заданий
- `total` - общее количество заданий, соответствующих фильтрам

**Возможные ошибки:**
- `400 Bad Request` - невалидные параметры пагинации
- `500 Internal Server Error` - ошибка при получении списка

---

### 5. Health Check

**GET** `/health`

Проверка работоспособности API.

**Ответ (200 OK):**
```
OK
```

## Примеры использования

### Создание задания с curl

```bash
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "execute_at": "2025-11-10T15:00:00Z",
    "task_type": "http_callback",
    "payload": {
        "url": "http://at-api:8080/health",
  	    "method": "GET",
        "data": {
            "param1":"value1"
        }
    }
    "max_attempts": 3
  }'
```

### Получение статуса задания

```bash
curl http://localhost:8080/api/v1/tasks/1
```

### Отмена задания

```bash
curl -X DELETE http://localhost:8080/api/v1/tasks/1
```

### Получение списка pending заданий

```bash
curl "http://localhost:8080/api/v1/tasks?status=pending&limit=10"
```

## Особенности

- Чистый Go без ORM и фреймворков
- Прямые SQL запросы через `database/sql`
- Валидация данных на уровне handlers
- Бизнес-логика в services
- Health check endpoint для мониторинга
- Поддержка Docker и локального запуска

## Тестирование

Интеграционные HTTP тесты для всех API endpoints. Стресс тест.

### Быстрый запуск тестов

```bash
# 1. Запустите API
cd src && go run main.go &

# 2. Запустите тесты (в другом терминале)
cd tests && go test -v
```

### Что тестируется

- ✅ POST /api/v1/tasks - создание задания
- ✅ GET /api/v1/tasks/:id - получение задания
- ✅ DELETE /api/v1/tasks/:id - отмена задания
- ✅ GET /api/v1/tasks - список заданий с фильтрами и пагинацией
- ✅ GET /health - healthcheck
- ✅ Полный цикл: создание → получение → отмена
- ✅ Стресс тест на 4000 заданий

### Документация

- [TEST_README.md](TEST_README.md) - подробная документация по тестам
- [tests/README.md](tests/README.md) - быстрый старт

**Важно:** Тесты работают через HTTP с реальной БД. Перед запуском убедитесь, что PostgreSQL запущен и миграции применены.