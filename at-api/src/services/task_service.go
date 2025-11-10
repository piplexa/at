// Package services содержит бизнес-логику приложения.
// TaskService предоставляет методы для работы с запланированными заданиями:
// создание, получение, отмена и получение списка заданий.
// Взаимодействует напрямую с базой данных через sql.DB.
package services

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"at-api/models"
)

var (
	// ErrTaskNotFound возвращается, когда задание с указанным ID не найдено
	ErrTaskNotFound = errors.New("task not found")
	// ErrInvalidExecuteTime возвращается, когда время выполнения задания в прошлом
	ErrInvalidExecuteTime = errors.New("execute_at must be in the future")
)

// TaskService предоставляет методы для управления заданиями
type TaskService struct {
	db *sql.DB
}

// NewTaskService создает новый экземпляр TaskService.
// Параметры:
//   - db: указатель на пул подключений к базе данных
func NewTaskService(db *sql.DB) *TaskService {
	return &TaskService{db: db}
}

// CreateTask создает новое запланированное задание в базе данных.
// Параметры:
//   - req: данные для создания задания (execute_at, task_type, payload, max_attempts)
//
// Возвращает созданное задание или ошибку.
// Валидирует, что execute_at не в прошлом.
func (s *TaskService) CreateTask(req *models.CreateTaskRequest) (*models.ScheduledTask, error) {
	// Валидация: время выполнения не должно быть в прошлом
	if req.ExecuteAt.Before(time.Now()) {
		return nil, ErrInvalidExecuteTime
	}

	// Устанавливаем значение по умолчанию для max_attempts
	maxAttempts := req.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = 3
	}

	query := `
		INSERT INTO scheduled_tasks (execute_at, task_type, payload, max_attempts)
		VALUES ($1, $2, $3, $4)
		RETURNING id, execute_at, task_type, payload, status, attempts, max_attempts,
		          error_message, created_at, updated_at, completed_at
	`

	task := &models.ScheduledTask{}
	err := s.db.QueryRow(
		query,
		req.ExecuteAt,
		req.TaskType,
		req.Payload,
		maxAttempts,
	).Scan(
		&task.ID,
		&task.ExecuteAt,
		&task.TaskType,
		&task.Payload,
		&task.Status,
		&task.Attempts,
		&task.MaxAttempts,
		&task.ErrorMessage,
		&task.CreatedAt,
		&task.UpdatedAt,
		&task.CompletedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	return task, nil
}

// GetTask получает задание по его ID.
// Параметры:
//   - id: идентификатор задания
//
// Возвращает задание или ошибку ErrTaskNotFound, если задание не найдено.
func (s *TaskService) GetTask(id int64) (*models.ScheduledTask, error) {
	query := `
		SELECT id, execute_at, task_type, payload, status, attempts, max_attempts,
		       error_message, created_at, updated_at, completed_at
		FROM scheduled_tasks
		WHERE id = $1
	`

	task := &models.ScheduledTask{}
	err := s.db.QueryRow(query, id).Scan(
		&task.ID,
		&task.ExecuteAt,
		&task.TaskType,
		&task.Payload,
		&task.Status,
		&task.Attempts,
		&task.MaxAttempts,
		&task.ErrorMessage,
		&task.CreatedAt,
		&task.UpdatedAt,
		&task.CompletedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrTaskNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	return task, nil
}

// CancelTask отменяет задание, устанавливая его статус в 'cancelled'.
// Параметры:
//   - id: идентификатор задания
//
// Возвращает обновленное задание или ошибку ErrTaskNotFound, если задание не найдено.
// Можно отменить только задания в статусе 'pending' или 'processing'.
func (s *TaskService) CancelTask(id int64) (*models.ScheduledTask, error) {
	query := `
		UPDATE scheduled_tasks
		SET status = 'cancelled'
		WHERE id = $1 AND status IN ('pending', 'processing')
		RETURNING id, execute_at, task_type, payload, status, attempts, max_attempts,
		          error_message, created_at, updated_at, completed_at
	`

	task := &models.ScheduledTask{}
	err := s.db.QueryRow(query, id).Scan(
		&task.ID,
		&task.ExecuteAt,
		&task.TaskType,
		&task.Payload,
		&task.Status,
		&task.Attempts,
		&task.MaxAttempts,
		&task.ErrorMessage,
		&task.CreatedAt,
		&task.UpdatedAt,
		&task.CompletedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrTaskNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to cancel task: %w", err)
	}

	return task, nil
}

// ListTasks возвращает список заданий с фильтрацией и пагинацией.
// Параметры:
//   - params: параметры фильтрации (status, task_type, limit, offset)
//
// Возвращает массив заданий и общее количество заданий, соответствующих фильтрам.
func (s *TaskService) ListTasks(params models.ListTasksParams) ([]models.ScheduledTask, int, error) {
	// Устанавливаем значения по умолчанию для пагинации
	if params.Limit == 0 {
		params.Limit = 50
	}
	if params.Limit > 100 {
		params.Limit = 100
	}

	// Строим запрос с учетом фильтров
	query := `
		SELECT id, execute_at, task_type, payload, status, attempts, max_attempts,
		       error_message, created_at, updated_at, completed_at
		FROM scheduled_tasks
		WHERE 1=1
	`
	countQuery := `SELECT COUNT(*) FROM scheduled_tasks WHERE 1=1`
	args := []interface{}{}
	argPos := 1

	// Добавляем фильтр по статусу
	if params.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argPos)
		countQuery += fmt.Sprintf(" AND status = $%d", argPos)
		args = append(args, params.Status)
		argPos++
	}

	// Добавляем фильтр по типу задания
	if params.TaskType != "" {
		query += fmt.Sprintf(" AND task_type = $%d", argPos)
		countQuery += fmt.Sprintf(" AND task_type = $%d", argPos)
		args = append(args, params.TaskType)
		argPos++
	}

	// Получаем общее количество записей
	var total int
	err := s.db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count tasks: %w", err)
	}

	// Добавляем сортировку и пагинацию
	query += " ORDER BY created_at DESC"
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argPos, argPos+1)
	args = append(args, params.Limit, params.Offset)

	// Выполняем запрос
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list tasks: %w", err)
	}
	defer rows.Close()

	// Читаем результаты
	tasks := []models.ScheduledTask{}
	for rows.Next() {
		var task models.ScheduledTask
		err := rows.Scan(
			&task.ID,
			&task.ExecuteAt,
			&task.TaskType,
			&task.Payload,
			&task.Status,
			&task.Attempts,
			&task.MaxAttempts,
			&task.ErrorMessage,
			&task.CreatedAt,
			&task.UpdatedAt,
			&task.CompletedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan task: %w", err)
		}
		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating tasks: %w", err)
	}

	return tasks, total, nil
}
