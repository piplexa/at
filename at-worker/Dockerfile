# Dockerfile для at-worker сервиса
# Многоступенчатый build для минимизации размера финального образа

# Стадия 1: Сборка приложения
FROM golang:1.22-alpine AS builder

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем go.mod и go.sum для кэширования зависимостей
COPY at-worker/src/go.mod at-worker/src/go.sum ./

# Загружаем зависимости
RUN go mod download

# Копируем исходный код
COPY at-worker/src/ ./

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Стадия 2: Финальный образ
FROM alpine:latest

# Устанавливаем ca-certificates для HTTPS запросов
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Копируем скомпилированный бинарник из builder стадии
COPY --from=builder /app/main .

# Копируем .env файл (если есть) - опционально, лучше использовать переменные окружения
COPY at-worker/.env .env

# Запускаем приложение
CMD ["./main"]

