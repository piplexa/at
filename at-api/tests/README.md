# Просто попробовал claude - сделай тесты..... Работает, но потом разберусь

# Быстрый старт

## Запуск тестов

### 1. Убедитесь, что PostgreSQL запущен и БД настроена

```bash
# Проверьте подключение
psql -U postgres -d at_scheduler -c "SELECT COUNT(*) FROM scheduled_tasks;"
```

### 2. Запустите at-api сервис

```bash
# В отдельном терминале
cd ../src
go run main.go

# Проверьте, что API работает
curl http://localhost:8080/health
```

### 3. Запустите тесты

```bash
# В этой директории
go test -v
```

## Краткая команда

```bash
# Запустить всё одной строкой (из директории at-api)
cd src && go run main.go & sleep 2 && cd ../tests && go test -v
```

## Результат

Если всё работает, вы увидите:

```
=== RUN   TestHealthCheck
    api_test.go:64: Testing GET /health
    api_test.go:76: ✅ Health check passed
--- PASS: TestHealthCheck (0.01s)
...
PASS
ok      at-api/tests    0.210s
```

## Подробная документация

См. [TEST_README.md](../TEST_README.md)
