// Package db предоставляет функциональность для работы с базой данных PostgreSQL.
// Создает и управляет пулом подключений к БД, используя стандартный пакет database/sql
// и драйвер pq для PostgreSQL. Идентичен модулю db из at-api для единообразия.
package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq" // Драйвер PostgreSQL
)

// NewPostgresDB создает новое подключение к PostgreSQL и возвращает пул соединений.
// Параметры:
//   - dsn: строка подключения в формате "host=... port=... user=... password=... dbname=... sslmode=..."
//
// Возвращает указатель на sql.DB или ошибку при невозможности подключения.
// Также настраивает параметры пула соединений для оптимальной работы.
func NewPostgresDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть подключение к БД: %w", err)
	}

	// Настройка пула соединений
	db.SetMaxOpenConns(25)                 // Максимальное количество открытых соединений
	db.SetMaxIdleConns(5)                  // Максимальное количество простаивающих соединений
	db.SetConnMaxLifetime(5 * time.Minute) // Максимальное время жизни соединения

	// Проверка подключения
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("не удалось установить соединение с БД: %w", err)
	}

	return db, nil
}
