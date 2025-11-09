## Основные компоненты:

**main.go** - точка входа, инициализация, graceful shutdown

**config/config.go** - конфигурация через ENV (DB DSN, polling interval, worker ID, batch size)

**database/postgres.go** - пул соединений PostgreSQL

**worker/worker.go** - основной polling loop:
- SELECT заданий с FOR UPDATE SKIP LOCKED
- Атомарное обновление статуса на 'processing'
- Параллельный запуск executor через goroutines
- Обработка результатов

**worker/executor.go** - выполнение заданий:
- Роутинг по task_type
- HTTP callback к API-proxy/WebSocket
- Отправка в RabbitMQ
- Обработка ошибок и retry логика

**worker/cleaner.go** - отдельная goroutine:
- Каждые 5 минут ищет зависшие задания (status='processing' AND updated_at < NOW() - 5 min)
- Возвращает их в 'pending' с инкрементом attempts

**models/task.go** - структура ScheduledTask