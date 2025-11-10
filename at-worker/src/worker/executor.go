// Package worker содержит логику выполнения запланированных заданий.
// Файл executor.go отвечает за маршрутизацию и выполнение заданий в зависимости от их типа (task_type).
// Поддерживает различные типы выполнения: HTTP callback, отправку в RabbitMQ, и другие.
package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"at-worker/models"
)

// Executor отвечает за выполнение заданий различных типов
type Executor struct {
	httpClient *http.Client
}

// NewExecutor создает новый экземпляр Executor с настроенным HTTP клиентом.
// HTTP клиент используется для отправки callback-запросов к внешним API.
func NewExecutor() *Executor {
	return &Executor{
		httpClient: &http.Client{
			Timeout: 30 * time.Second, // Таймаут для HTTP запросов
		},
	}
}

// Execute выполняет задание в зависимости от его типа (task_type).
// Параметры:
//   - ctx: контекст для отмены выполнения
//   - task: задание для выполнения
//
// Возвращает результат выполнения (TaskResult) с информацией об успехе или ошибке.
// Поддерживаемые типы заданий:
//   - "http_callback": выполняет HTTP POST запрос к URL из payload
//   - "rabbitmq": отправляет сообщение в RabbitMQ (заглушка)
//   - "email": отправляет email (заглушка)
//   - другие типы: возвращают ошибку "unknown task type"
func (e *Executor) Execute(ctx context.Context, task *models.ScheduledTask) models.TaskResult {
	log.Printf("[Executor] Executing task %d (type: %s)", task.ID, task.TaskType)

	// Маршрутизация по типу задания
	switch task.TaskType {
	case "http_callback":
		return e.executeHTTPCallback(ctx, task)
	case "rabbitmq":
		return e.executeRabbitMQ(ctx, task)
	case "email":
		return e.executeEmail(ctx, task)
	default:
		return models.TaskResult{
			TaskID:       task.ID,
			Success:      false,
			ErrorMessage: fmt.Sprintf("unknown task type: %s", task.TaskType),
		}
	}
}

// executeHTTPCallback выполняет HTTP POST запрос к URL, указанному в payload.
// Ожидает, что payload содержит поля: {"url": "http://...", "data": {...}}
// Возвращает успех, если HTTP статус 2xx, иначе ошибку.
func (e *Executor) executeHTTPCallback(ctx context.Context, task *models.ScheduledTask) models.TaskResult {
	// Парсим payload
	var payload struct {
		URL  string                 `json:"url"`
		Data map[string]interface{} `json:"data"`
	}

	if err := json.Unmarshal(task.Payload, &payload); err != nil {
		return models.TaskResult{
			TaskID:       task.ID,
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to parse payload: %v", err),
		}
	}

	if payload.URL == "" {
		return models.TaskResult{
			TaskID:       task.ID,
			Success:      false,
			ErrorMessage: "url field is required in payload",
		}
	}

	// Подготовка данных для отправки
	jsonData, err := json.Marshal(payload.Data)
	if err != nil {
		return models.TaskResult{
			TaskID:       task.ID,
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to marshal data: %v", err),
		}
	}

	// Создание HTTP запроса
	req, err := http.NewRequestWithContext(ctx, "POST", payload.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		return models.TaskResult{
			TaskID:       task.ID,
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to create request: %v", err),
		}
	}

	req.Header.Set("Content-Type", "application/json")

	// Выполнение запроса
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return models.TaskResult{
			TaskID:       task.ID,
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to execute request: %v", err),
		}
	}
	defer resp.Body.Close()

	// Проверка статуса ответа
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return models.TaskResult{
			TaskID:       task.ID,
			Success:      false,
			ErrorMessage: fmt.Sprintf("HTTP request failed with status: %d", resp.StatusCode),
		}
	}

	log.Printf("[Executor] Task %d completed successfully (HTTP %d)", task.ID, resp.StatusCode)
	return models.TaskResult{
		TaskID:  task.ID,
		Success: true,
	}
}

// executeRabbitMQ отправляет сообщение в RabbitMQ очередь.
// Ожидает, что payload содержит поля: {"queue": "queue_name", "message": {...}}
// Примечание: это заглушка, требуется реализация подключения к RabbitMQ.
func (e *Executor) executeRabbitMQ(ctx context.Context, task *models.ScheduledTask) models.TaskResult {
	// TODO: Реализовать отправку в RabbitMQ
	// Для этого нужно:
	// 1. Установить соединение с RabbitMQ (амqp)
	// 2. Парсить payload для получения имени очереди и сообщения
	// 3. Отправить сообщение в очередь

	log.Printf("[Executor] RabbitMQ execution for task %d (not implemented yet)", task.ID)

	return models.TaskResult{
		TaskID:       task.ID,
		Success:      false,
		ErrorMessage: "RabbitMQ execution not implemented",
	}
}

// executeEmail отправляет email уведомление.
// Ожидает, что payload содержит поля: {"to": "email@example.com", "subject": "...", "body": "..."}
// Примечание: это заглушка, требуется реализация отправки email.
func (e *Executor) executeEmail(ctx context.Context, task *models.ScheduledTask) models.TaskResult {
	// TODO: Реализовать отправку email
	// Для этого нужно:
	// 1. Настроить SMTP клиент
	// 2. Парсить payload для получения адреса, темы и тела письма
	// 3. Отправить email через SMTP

	log.Printf("[Executor] Email execution for task %d (not implemented yet)", task.ID)

	return models.TaskResult{
		TaskID:       task.ID,
		Success:      false,
		ErrorMessage: "Email execution not implemented",
	}
}
