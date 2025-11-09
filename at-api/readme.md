# API

Endpoints:
POST /api/v1/tasks - создание задания (execute_at, task_type, payload, max_attempts)
GET /api/v1/tasks/:id - статус задания
DELETE /api/v1/tasks/:id - отмена задания (status='cancelled')
GET /api/v1/tasks - список заданий с фильтрами (status, task_type, пагинация)
handlers/ - HTTP обработчики с валидацией
services/task_service.go - бизнес-логика:

Валидация execute_at (не в прошлом)
INSERT в scheduled_tasks
UPDATE для отмены
SELECT для получения статуса

Особенности:

Легковесный HTTP сервер (gin/echo/chi)
JSON API
Graceful shutdown
Health check endpoint