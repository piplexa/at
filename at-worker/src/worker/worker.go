// Package worker содержит основную логику опроса и обработки запланированных заданий.
// Файл worker.go реализует главный polling loop, который:
// 1. Извлекает задания из БД с использованием FOR UPDATE SKIP LOCKED (для конкурентного доступа)
// 2. Атомарно обновляет статус заданий на 'processing'
// 3. Запускает goroutines для параллельного выполнения заданий через Executor
// 4. Обрабатывает результаты и обновляет статусы заданий в БД
//
// Механизм FOR UPDATE SKIP LOCKED гарантирует, что несколько worker'ов не получат одно и то же задание.
package worker

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"at-worker/models"
)

// Worker отвечает за опрос и обработку запланированных заданий
type Worker struct {
	db              *sql.DB
	executor        *Executor
	workerID        string
	pollingInterval time.Duration
	batchSize       int
}

// NewWorker создает новый экземпляр Worker.
// Параметры:
//   - db: подключение к базе данных
//   - workerID: уникальный идентификатор worker'а для логирования
//   - pollingInterval: интервал опроса БД для новых заданий
//   - batchSize: количество заданий, извлекаемых за один запрос
func NewWorker(db *sql.DB, workerID string, pollingInterval time.Duration, batchSize int) *Worker {
	return &Worker{
		db:              db,
		executor:        NewExecutor(),
		workerID:        workerID,
		pollingInterval: pollingInterval,
		batchSize:       batchSize,
	}
}

// Start запускает основной polling loop worker'а.
// Worker периодически (каждые pollingInterval) опрашивает БД на наличие заданий к выполнению.
// Использует FOR UPDATE SKIP LOCKED для безопасного конкурентного доступа нескольких worker'ов.
// Параметры:
//   - ctx: контекст для остановки worker'а при завершении работы приложения
func (w *Worker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.pollingInterval)
	defer ticker.Stop()

	log.Printf("[Worker %s] Started with polling interval %v, batch size %d", w.workerID, w.pollingInterval, w.batchSize)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[Worker %s] Shutting down...", w.workerID)
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

// processBatch извлекает пакет заданий из БД и обрабатывает их.
// Основные шаги:
// 1. SELECT заданий с FOR UPDATE SKIP LOCKED (конкурентная безопасность)
// 2. Атомарное обновление статуса на 'processing'
// 3. Параллельное выполнение заданий в goroutines
// 4. Обработка результатов и обновление статусов
func (w *Worker) processBatch(ctx context.Context) {
	// Начинаем транзакцию для атомарного захвата заданий
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("[Worker %s] Error starting transaction: %v", w.workerID, err)
		return
	}
	defer tx.Rollback()

	// КРИТИЧНО: Используем FOR UPDATE SKIP LOCKED для избежания конфликтов между worker'ами
	// SKIP LOCKED означает, что если строка уже заблокирована другим worker'ом, мы её пропускаем
	// Это гарантирует, что одно и то же задание не попадет в разные worker'ы
	query := `
		SELECT id, execute_at, task_type, payload, status, attempts, max_attempts,
		       error_message, created_at, updated_at, completed_at
		FROM scheduled_tasks
		WHERE status = 'pending'
		  AND execute_at <= NOW()
		ORDER BY execute_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`

	rows, err := tx.QueryContext(ctx, query, w.batchSize)
	if err != nil {
		log.Printf("[Worker %s] Error querying tasks: %v", w.workerID, err)
		return
	}
	defer rows.Close()

	// Читаем задания и сразу обновляем их статус на 'processing'
	var tasks []*models.ScheduledTask
	var taskIDs []int64

	for rows.Next() {
		task := &models.ScheduledTask{}
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
			log.Printf("[Worker %s] Error scanning task: %v", w.workerID, err)
			continue
		}

		tasks = append(tasks, task)
		taskIDs = append(taskIDs, task.ID)
	}

	if err := rows.Err(); err != nil {
		log.Printf("[Worker %s] Error iterating rows: %v", w.workerID, err)
		return
	}

	if len(tasks) == 0 {
		// Нет заданий для обработки
		return
	}

	log.Printf("[Worker %s] Found %d tasks to process", w.workerID, len(tasks))

	// Атомарно обновляем статус всех захваченных заданий на 'processing'
	// Это важно сделать в той же транзакции, чтобы гарантировать атомарность
	// Формируем плейсхолдеры для IN clause
	placeholders := make([]string, len(taskIDs))
	args := make([]interface{}, len(taskIDs))
	for i, id := range taskIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	updateQuery := fmt.Sprintf(`
		UPDATE scheduled_tasks
		SET status = 'processing',
		    attempts = attempts + 1
		WHERE id IN (%s)
	`, strings.Join(placeholders, ", "))

	_, err = tx.ExecContext(ctx, updateQuery, args...)
	if err != nil {
		log.Printf("[Worker %s] Error updating task status: %v", w.workerID, err)
		return
	}

	// Коммитим транзакцию - задания теперь принадлежат этому worker'у
	if err := tx.Commit(); err != nil {
		log.Printf("[Worker %s] Error committing transaction: %v", w.workerID, err)
		return
	}

	// Выполняем задания параллельно в goroutines
	w.executeTasks(ctx, tasks)
}

// executeTasks выполняет задания параллельно в goroutines и обрабатывает результаты.
// Использует WaitGroup для ожидания завершения всех goroutines.
// После выполнения обновляет статусы заданий в БД на основе результатов.
func (w *Worker) executeTasks(ctx context.Context, tasks []*models.ScheduledTask) {
	var wg sync.WaitGroup
	resultsChan := make(chan models.TaskResult, len(tasks))

	// Запускаем goroutine для каждого задания
	for _, task := range tasks {
		wg.Add(1)
		go func(t *models.ScheduledTask) {
			defer wg.Done()

			// Создаем контекст с таймаутом для выполнения задания
			taskCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()

			// Выполняем задание через Executor
			result := w.executor.Execute(taskCtx, t)
			resultsChan <- result
		}(task)
	}

	// Ждем завершения всех goroutines
	wg.Wait()
	close(resultsChan)

	// Обрабатываем результаты
	for result := range resultsChan {
		w.handleTaskResult(ctx, result)
	}
}

// handleTaskResult обрабатывает результат выполнения задания и обновляет его статус в БД.
// Если выполнение успешно - статус 'completed'
// Если ошибка и не исчерпаны попытки - статус 'pending' (для retry)
// Если ошибка и исчерпаны попытки - статус 'failed'
func (w *Worker) handleTaskResult(ctx context.Context, result models.TaskResult) {
	if result.Success {
		// Задание выполнено успешно
		query := `
			UPDATE scheduled_tasks
			SET status = 'completed',
			    completed_at = NOW(),
			    error_message = $2
			WHERE id = $1
		`
		_, err := w.db.ExecContext(ctx, query, result.TaskID, result.ErrorMessage)
		if err != nil {
			log.Printf("[Worker %s] Error updating completed task %d: %v", w.workerID, result.TaskID, err)
			return
		}
		log.Printf("[Worker %s] Task %d completed successfully", w.workerID, result.TaskID)
	} else {
		// Задание завершилось с ошибкой
		// Проверяем, можно ли повторить попытку
		var attempts, maxAttempts int
		checkQuery := `SELECT attempts, max_attempts FROM scheduled_tasks WHERE id = $1`
		err := w.db.QueryRowContext(ctx, checkQuery, result.TaskID).Scan(&attempts, &maxAttempts)
		if err != nil {
			log.Printf("[Worker %s] Error checking attempts for task %d: %v", w.workerID, result.TaskID, err)
			return
		}

		if attempts >= maxAttempts {
			// Исчерпаны попытки - помечаем как failed
			query := `
				UPDATE scheduled_tasks
				SET status = 'failed',
				    error_message = $2,
				    completed_at = NOW()
				WHERE id = $1
			`
			_, err := w.db.ExecContext(ctx, query, result.TaskID, result.ErrorMessage)
			if err != nil {
				log.Printf("[Worker %s] Error updating failed task %d: %v", w.workerID, result.TaskID, err)
				return
			}
			log.Printf("[Worker %s] Task %d failed (max attempts reached): %s", w.workerID, result.TaskID, result.ErrorMessage)
		} else {
			// Еще есть попытки - возвращаем в pending для retry
			query := `
				UPDATE scheduled_tasks
				SET status = 'pending',
				    error_message = $2
				WHERE id = $1
			`
			_, err := w.db.ExecContext(ctx, query, result.TaskID, result.ErrorMessage)
			if err != nil {
				log.Printf("[Worker %s] Error updating task %d for retry: %v", w.workerID, result.TaskID, err)
				return
			}
			log.Printf("[Worker %s] Task %d failed (attempt %d/%d), will retry: %s", w.workerID, result.TaskID, attempts, maxAttempts, result.ErrorMessage)
		}
	}
}
