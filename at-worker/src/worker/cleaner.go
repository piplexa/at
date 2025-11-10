// Package worker содержит логику очистки и восстановления зависших заданий.
// Файл cleaner.go отвечает за периодический поиск заданий, которые застряли в статусе 'processing'
// и возвращает их обратно в статус 'pending' для повторной обработки.
// Это критично для отказоустойчивости системы при падении worker'ов.
package worker

import (
	"context"
	"database/sql"
	"log"
	"time"
)

// Cleaner отвечает за поиск и восстановление зависших заданий
type Cleaner struct {
	db              *sql.DB
	cleanerInterval time.Duration // Интервал между запусками cleaner'а
	stuckTimeout    time.Duration // Время, после которого задание считается зависшим
}

// NewCleaner создает новый экземпляр Cleaner.
// Параметры:
//   - db: подключение к базе данных
//   - cleanerInterval: интервал между проверками зависших заданий
//   - stuckTimeout: время, после которого задание в статусе 'processing' считается зависшим
func NewCleaner(db *sql.DB, cleanerInterval, stuckTimeout time.Duration) *Cleaner {
	return &Cleaner{
		db:              db,
		cleanerInterval: cleanerInterval,
		stuckTimeout:    stuckTimeout,
	}
}

// Start запускает cleaner в отдельной goroutine.
// Cleaner периодически (каждые cleanerInterval) проверяет наличие зависших заданий
// и возвращает их в статус 'pending' с инкрементом счетчика попыток.
// Параметры:
//   - ctx: контекст для остановки cleaner'а при завершении работы приложения
func (c *Cleaner) Start(ctx context.Context) {
	ticker := time.NewTicker(c.cleanerInterval)
	defer ticker.Stop()

	log.Printf("[Cleaner] Started with interval %v, stuck timeout %v", c.cleanerInterval, c.stuckTimeout)

	// Сразу выполняем первую проверку
	c.cleanStuckTasks(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("[Cleaner] Shutting down...")
			return
		case <-ticker.C:
			c.cleanStuckTasks(ctx)
		}
	}
}

// cleanStuckTasks ищет зависшие задания и возвращает их в статус 'pending'.
// Зависшим считается задание, которое находится в статусе 'processing'
// и не обновлялось дольше, чем stuckTimeout.
// Для каждого зависшего задания:
//   - Статус меняется на 'pending'
//   - Инкрементируется счетчик попыток (attempts)
//   - Если достигнут max_attempts, задание переводится в статус 'failed'
func (c *Cleaner) cleanStuckTasks(ctx context.Context) {
	// SQL запрос для поиска и обновления зависших заданий
	// Задание считается зависшим, если:
	// 1. Статус = 'processing'
	// 2. updated_at < NOW() - stuckTimeout
	// 3. attempts < max_attempts
	query := `
		UPDATE scheduled_tasks
		SET status = 'pending',
		    attempts = attempts + 1
		WHERE id IN (
			SELECT id
			FROM scheduled_tasks
			WHERE status = 'processing'
			  AND updated_at < NOW() - INTERVAL '1 second' * $1
			  AND attempts < max_attempts
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, attempts, max_attempts
	`

	rows, err := c.db.QueryContext(ctx, query, int(c.stuckTimeout.Seconds()))
	if err != nil {
		log.Printf("[Cleaner] Error cleaning stuck tasks: %v", err)
		return
	}
	defer rows.Close()

	restoredCount := 0
	for rows.Next() {
		var id int64
		var attempts, maxAttempts int
		if err := rows.Scan(&id, &attempts, &maxAttempts); err != nil {
			log.Printf("[Cleaner] Error scanning row: %v", err)
			continue
		}
		restoredCount++
		log.Printf("[Cleaner] Restored stuck task %d (attempt %d/%d)", id, attempts, maxAttempts)
	}

	if err := rows.Err(); err != nil {
		log.Printf("[Cleaner] Error iterating rows: %v", err)
		return
	}

	// Дополнительно помечаем как failed задания, которые исчерпали попытки
	failQuery := `
		UPDATE scheduled_tasks
		SET status = 'failed',
		    error_message = 'Max attempts reached',
		    completed_at = NOW()
		WHERE id IN (
			SELECT id
			FROM scheduled_tasks
			WHERE status = 'processing'
			  AND updated_at < NOW() - INTERVAL '1 second' * $1
			  AND attempts >= max_attempts
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id
	`

	failRows, err := c.db.QueryContext(ctx, failQuery, int(c.stuckTimeout.Seconds()))
	if err != nil {
		log.Printf("[Cleaner] Error marking failed tasks: %v", err)
		return
	}
	defer failRows.Close()

	failedCount := 0
	for failRows.Next() {
		var id int64
		if err := failRows.Scan(&id); err != nil {
			log.Printf("[Cleaner] Error scanning failed row: %v", err)
			continue
		}
		failedCount++
		log.Printf("[Cleaner] Marked task %d as failed (max attempts reached)", id)
	}

	if restoredCount > 0 || failedCount > 0 {
		log.Printf("[Cleaner] Cleanup complete: restored %d tasks, failed %d tasks", restoredCount, failedCount)
	}
}
