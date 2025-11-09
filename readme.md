# AT (Планировщик заданий) - архитектурное решение

## Почему собственное решение

**Требования:**
- До 100,000 заданий одновременно
- Точность выполнения до секунды
- Критичность: потеря заданий недопустима
- Разовые задания (не cron)

**Готовые решения не подходят:**
- **Temporal.io** - платный, избыточен (workflow engine не нужен)
- **RabbitMQ Delayed Plugin** - хранит в RAM, теряет задания при падении
- **Unix at/cron** - не масштабируется, нет точности до секунды
- **Airflow/Celery** - для ETL, точность ±5-10 секунд
- **LISTEN/NOTIFY PostgreSQL** - ненадежен (теряет уведомления при разрыве соединения)

**Решение:** собственный AT на PostgreSQL с polling

## Архитектура: API → PostgreSQL ← Workers×N

```
Back-end (API) 
    ↓ INSERT
PostgreSQL (отдельный сервер для надежности)
    ↑ SELECT (polling каждую секунду)
Workers×N (Golang, горизонтальное масштабирование)
```

## Таблица scheduled_tasks

```sql
CREATE TABLE scheduled_tasks (
    id BIGSERIAL PRIMARY KEY,
    execute_at TIMESTAMP NOT NULL,           -- Когда выполнить
    task_type VARCHAR(50) NOT NULL,          -- Тип задания
    payload JSONB NOT NULL,                  -- Данные для выполнения
    status VARCHAR(20) DEFAULT 'pending',    -- pending|processing|completed|failed|cancelled
    attempts INT DEFAULT 0,                  -- Счетчик попыток
    max_attempts INT DEFAULT 3,              -- Лимит retry
    error_message TEXT,                      -- Ошибка если failed
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    completed_at TIMESTAMP
);

CREATE INDEX idx_pending_tasks 
ON scheduled_tasks(execute_at, status) 
WHERE status IN ('pending', 'processing');
```

## Алгоритм работы Worker (Golang)

**Почему Golang:**
- Легковесные goroutines для параллельного выполнения заданий
- Нативная поддержка PostgreSQL
- Высокая производительность при минимальном потреблении ресурсов
- Простота деплоя (один бинарник)

**Алгоритм:**

1. **Polling каждую секунду:**
```sql
SELECT * FROM scheduled_tasks 
WHERE execute_at <= NOW() 
  AND status = 'pending'
ORDER BY execute_at
FOR UPDATE SKIP LOCKED  -- Защита от дублирования между workers
LIMIT 1000;
```

2. **Атомарное взятие в работу:**
```sql
BEGIN;
UPDATE scheduled_tasks 
SET status='processing', updated_at=NOW()
WHERE id IN (selected_ids);
COMMIT;  -- Короткая транзакция!
```

3. **Выполнение задания** (вне транзакции):
   - HTTP callback к API-proxy/WebSocket
   - Или прямая команда в RabbitMQ

4. **Обновление результата:**
   - Успех: `status='completed', completed_at=NOW()`
   - Ошибка: `status='failed', attempts++, error_message`
   - Retry: если `attempts < max_attempts` → `status='pending'`

5. **Защита от зависших заданий** (отдельная горутина):
```sql
-- Каждые 5 минут возвращаем зависшие задания
UPDATE scheduled_tasks 
SET status='pending', attempts=attempts+1
WHERE status='processing' 
  AND updated_at < NOW() - INTERVAL '5 minutes';
```

**Ключевые особенности:**
- `FOR UPDATE SKIP LOCKED` гарантирует: одно задание = один worker
- Короткие транзакции (только SELECT+UPDATE status)
- Горизонтальное масштабирование (N workers без координации)
- Простота: polling надежнее event-driven для критичных систем
- Предсказуемая нагрузка: 10 workers × 1 req/sec = 10 запросов/сек на PostgreSQL