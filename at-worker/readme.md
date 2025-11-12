# AT Worker - Worker-сервис для обработки запланированных заданий

## Основные компоненты

**main.go** - точка входа, инициализация, graceful shutdown

**config/config.go** - конфигурация через ENV (DB DSN, polling interval, worker ID, batch size)

**db/postgres.go** - пул соединений PostgreSQL

**worker/worker.go** - основной polling loop:
- SELECT заданий с FOR UPDATE SKIP LOCKED (гарантирует, что одно задание не попадет в разные worker'ы)
- Атомарное обновление статуса на 'processing'
- Параллельный запуск executor через goroutines
- Обработка результатов

**worker/executor.go** - выполнение заданий:
- Роутинг по task_type
- HTTP callback к внешним API
- Отправка в RabbitMQ (заглушка)
- Email уведомления (заглушка)
- Обработка ошибок и retry логика

**task_type** - способ выполнения задания. Может принимать следующие значения:
- http_callback
- rabbitmq
- email

**http_callback** в базе данных - это json следующего формата:
```json
{"url": "http://test.com/", "data": {"param1":"data1"}}
```

*data* может быть как json строка, которая будет передана как raw в POST запросе. 


**worker/cleaner.go** - отдельная goroutine:
- Каждые 5 минут ищет зависшие задания (status='processing' AND updated_at < NOW() - 5 min)
- Возвращает их в 'pending' с инкрементом attempts
- Помечает как 'failed' задания, исчерпавшие попытки

**models/task.go** - структура ScheduledTask

## Запуск сервиса

### Локальный запуск

1. Создайте файл `.env` на основе примера выше
2. Запустите сервис:
```bash
cd at-worker/src
go run main.go
```

### Запуск через Docker

```bash
docker build -t at-worker -f at-worker/Dockerfile .
docker run --env-file at-worker/.env at-worker
```

### Запуск нескольких экземпляров

Для горизонтального масштабирования можно запустить несколько worker'ов.

#### Локальный запуск нескольких worker'ов

```bash
WORKER_ID=worker-1 go run main.go &
WORKER_ID=worker-2 go run main.go &
WORKER_ID=worker-3 go run main.go &
```

#### Масштабирование через docker-compose

Worker автоматически использует hostname контейнера как WORKER_ID, если переменная не задана явно:

```bash
docker-compose up --scale at-worker=3 -d
```

**Что произойдет:**
- Каждый контейнер получит уникальный hostname (например: `at-worker_1`, `at-worker_2`, `at-worker_3`)
- Worker автоматически использует hostname как WORKER_ID
- В логах будет видно: `[Worker at-worker_1]`, `[Worker at-worker_2]`, `[Worker at-worker_3]`

**Важно:**
- Переменная `WORKER_ID` в .env должна быть **закомментирована** или **не задана**
- Если задать `WORKER_ID=worker-1` в .env, все экземпляры получат одинаковый ID (плохо для диагностики)

**Механизм `FOR UPDATE SKIP LOCKED` гарантирует**, что разные worker'ы не будут обрабатывать одно и то же задание одновременно, независимо от WORKER_ID.

## Конфигурация

Все настройки задаются через переменные окружения или через файл `.env`.

### Загрузка конфигурации

Worker поддерживает несколько способов задания конфигурации (в порядке приоритета):

1. **Переменные окружения** (высший приоритет) - переопределяют все остальные способы
2. **Файл .env** - автоматически загружается, если существует в:
   - Текущей директории (`.env`)
   - Директории `/app/.env` (для Docker контейнера)

### Переменные окружения

| Переменная | Описание | Значение по умолчанию |
|-----------|----------|----------------------|
| DB_HOST | Хост PostgreSQL | localhost |
| DB_PORT | Порт PostgreSQL | 5432 |
| DB_USER | Пользователь БД | postgres |
| DB_PASSWORD | Пароль БД | postgres |
| DB_NAME | Имя БД | at_scheduler |
| DB_SSLMODE | Режим SSL | disable |
| WORKER_ID | ID для логов (опционально) | hostname контейнера |
| WORKER_POLLING_INTERVAL | Интервал опроса (сек) | 5 |
| WORKER_BATCH_SIZE | Размер батча заданий | 10 |
| WORKER_CLEANER_INTERVAL | Интервал cleaner (мин) | 5 |
| WORKER_STUCK_TIMEOUT | Таймаут зависания (мин) | 5 |

## Диагностика и отладка

### Проверка работоспособности

1. **Проверка подключения к БД**: при запуске в логах должно быть сообщение `Successfully connected to database`
2. **Проверка запуска Worker**: в логах `[Worker worker-1] Started with polling interval...`
3. **Проверка запуска Cleaner**: в логах `[Cleaner] Started with interval...`

### Типичные проблемы и их решение

#### Worker не подключается к БД

**Симптомы**: в логах ошибка `Failed to connect to database`

**Что проверить**:
- Правильность параметров подключения в `.env`
- Доступность PostgreSQL (проверить через `psql`)
- Применена ли DDL схема из `sql/ddl.sql`

**Где смотреть**:
```bash
psql -h localhost -U postgres -d at_scheduler -c "SELECT version();"
```

#### Worker не обрабатывает задания

**Симптомы**: задания есть в БД со статусом 'pending', но не обрабатываются

**Что проверить**:
1. Проверить, что `execute_at <= NOW()`:
```sql
SELECT id, execute_at, status, task_type
FROM scheduled_tasks
WHERE status = 'pending'
ORDER BY execute_at;
```

2. Проверить логи worker'а на наличие ошибок при извлечении заданий

3. Проверить, не заблокированы ли задания другим worker'ом:
```sql
SELECT * FROM pg_locks WHERE relation = 'scheduled_tasks'::regclass;
```

**Где смотреть**: логи worker'а покажут `[Worker worker-1] Found N tasks to process`

#### Задания зависают в статусе 'processing'

**Симптомы**: задания застревают в статусе 'processing' и не переходят в 'completed' или 'failed'

**Что проверить**:
1. Работает ли Cleaner (должны быть логи каждые 5 минут)
2. Проверить зависшие задания:
```sql
SELECT id, task_type, status, attempts, max_attempts, updated_at,
       NOW() - updated_at as stuck_duration
FROM scheduled_tasks
WHERE status = 'processing'
  AND updated_at < NOW() - INTERVAL '5 minutes'
ORDER BY updated_at;
```

3. Проверить логи на ошибки выполнения (timeout, ошибки HTTP запросов)

**Где смотреть**: логи Cleaner'а покажут `[Cleaner] Restored stuck task...` или `[Cleaner] Marked task ... as failed`

#### HTTP callback не выполняется

**Симптомы**: задания с типом `http_callback` падают с ошибкой

**Что проверить**:
1. Формат payload - должен содержать поля `url` и `data`:
```json
{
  "url": "http://example.com/webhook",
  "data": {"key": "value"}
}
```

2. Доступность целевого URL (проверить через curl)

3. Таймаут выполнения (по умолчанию 30 секунд для HTTP, 5 минут для задания)

**Где смотреть**:
```sql
SELECT id, task_type, payload, error_message
FROM scheduled_tasks
WHERE task_type = 'http_callback'
  AND status = 'failed';
```

Логи executor'а покажут детали ошибки: `[Executor] Task X failed: ...`

#### Одно и то же задание выполняется несколько раз

**Симптомы**: задание обрабатывается multiple раза разными worker'ами

**Возможные причины**:
- Это не должно происходить благодаря `FOR UPDATE SKIP LOCKED`
- Если происходит, проверить версию PostgreSQL (должна быть >= 9.5)

**Где смотреть**:
```sql
-- Проверить версию PostgreSQL
SELECT version();

-- Проверить дубликаты в логах
-- Поискать в логах одинаковые task ID от разных worker'ов
```

#### Медленная обработка заданий

**Симптомы**: большая очередь заданий, worker не успевает обрабатывать

**Что сделать**:
1. Увеличить `WORKER_BATCH_SIZE` (количество заданий за один опрос)
2. Уменьшить `WORKER_POLLING_INTERVAL` (чаще опрашивать БД)
3. Запустить больше worker'ов (горизонтальное масштабирование)

**Где смотреть**:
```sql
-- Посчитать очередь заданий
SELECT status, COUNT(*)
FROM scheduled_tasks
GROUP BY status;

-- Проверить среднее время обработки
SELECT task_type,
       AVG(EXTRACT(EPOCH FROM (completed_at - execute_at))) as avg_seconds
FROM scheduled_tasks
WHERE status = 'completed'
  AND completed_at IS NOT NULL
GROUP BY task_type;
```

### Мониторинг

Рекомендуемые метрики для мониторинга:

1. **Количество заданий по статусам**:
```sql
SELECT status, COUNT(*) FROM scheduled_tasks GROUP BY status;
```

2. **Зависшие задания**:
```sql
SELECT COUNT(*) FROM scheduled_tasks
WHERE status = 'processing'
  AND updated_at < NOW() - INTERVAL '5 minutes';
```

3. **Процент успешно выполненных заданий**:
```sql
SELECT
  ROUND(100.0 * SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) / COUNT(*), 2) as success_rate
FROM scheduled_tasks
WHERE status IN ('completed', 'failed');
```

4. **Задания с максимальным количеством попыток**:
```sql
SELECT id, task_type, attempts, max_attempts, error_message
FROM scheduled_tasks
WHERE attempts >= max_attempts - 1
  AND status != 'completed'
ORDER BY updated_at DESC
LIMIT 10;
```

### Graceful Shutdown

Worker поддерживает корректное завершение работы:

1. При получении `SIGINT` (Ctrl+C) или `SIGTERM` (docker stop):
   - Останавливается polling loop
   - Останавливается cleaner
   - Текущие выполняющиеся задания завершаются
   - Закрывается подключение к БД

2. В логах: `Received signal terminated, initiating graceful shutdown...`

3. Не рекомендуется использовать `SIGKILL` - задания могут остаться в статусе 'processing'