# Dockerfile для at-api сервиса
# Многоступенчатый build для минимизации размера финального образа

# Стадия 1: Сборка приложения
FROM golang:1.21-alpine AS builder

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем go.mod и go.sum для кэширования зависимостей
COPY at-api/src/go.mod at-api/src/go.sum ./

# Загружаем зависимости
RUN go mod download

# Копируем исходный код
COPY at-api/src/ ./

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Стадия 2: Финальный образ
FROM alpine:latest

# Устанавливаем ca-certificates для HTTPS запросов
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Копируем скомпилированный бинарник из builder стадии
COPY --from=builder /app/main .

# Открываем порт для API
EXPOSE 8080

# Запускаем приложение
CMD ["./main"]
